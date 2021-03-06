package main

import (
	"./dialects"
	"./payload"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/golang/protobuf/proto"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var config *Config
var jobQueue chan *Job
var storageClient dialects.StorageClient
var verbose bool
var isTerminating = false
var dispatcher *Dispatcher
var JobQueue chan Job
var Version string = "1.0dev" // Current version

// Returns the request's signature
func GetSignature(body []byte, time string) string {
	bodyHash := md5.New()
	io.WriteString(bodyHash, string(body[:]))

	requestHash := sha256.New()
	io.WriteString(requestHash, time)
	io.WriteString(requestHash, "|")
	io.WriteString(requestHash, hex.EncodeToString(bodyHash.Sum(nil)))
	io.WriteString(requestHash, "|")
	io.WriteString(requestHash, config.SharedSecret)

	return base64.StdEncoding.EncodeToString(requestHash.Sum(nil))
}

// Returns the protobuf message's session
func GetSession(c *payload.Collection) string {
	session := md5.New()
	io.WriteString(session, c.GetDeviceId())
	io.WriteString(session, ":")
	io.WriteString(session, c.GetClientId())
	io.WriteString(session, ":")
	io.WriteString(session, c.GetSystemVersion())
	io.WriteString(session, ":")
	io.WriteString(session, c.GetProductVersion())
	return hex.EncodeToString(session.Sum(nil))
}

// Prints the error messages.
func BroadcastError(w http.ResponseWriter, err string, code int) {
	log.Println(err)
	if verbose {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		fmt.Fprintf(w, `{"error":%q}`, err)
	}
}

// Controller for `/api/v1/track`
func TrackHandler(w http.ResponseWriter, r *http.Request) {
	// Do not accept new events while the server is shutting down.
	if isTerminating {
		BroadcastError(w, "Server is currenly shutting down", http.StatusServiceUnavailable)
	}

	// Ignore not POST messages.
	if r.Method != "POST" {
		BroadcastError(w, "Sending method is not POST", http.StatusMethodNotAllowed)
		return
	}

	// If the client did not send time, we ignore
	if r.Header.Get("X-Hamustro-Time") == "" {
		BroadcastError(w, "X-Hamustro-Time header is missing", http.StatusMethodNotAllowed)
		return
	}

	// If the client did not send signature of the message, we ignore
	if r.Header.Get("X-Hamustro-Signature") == "" {
		BroadcastError(w, "X-Hamustro-Signature header is missing", http.StatusMethodNotAllowed)
	}

	// Read the requests body into a variable.
	body, _ := ioutil.ReadAll(r.Body)

	// Calculate the request's signature
	if r.Header.Get("X-Hamustro-Signature") != GetSignature(body, r.Header.Get("X-Hamustro-Time")) {
		BroadcastError(w, "X-Hamustro-Signature header is invalid", http.StatusMethodNotAllowed)
		return
	}

	// Read the body into protobuf decoding.
	collection := &payload.Collection{}
	if err := proto.Unmarshal(body, collection); err != nil {
		BroadcastError(w, fmt.Sprintf("Unmarshaling protobuf collection is failed: %s", err.Error()), http.StatusBadRequest)
		return
	}

	// Checks the session information
	if GetSession(collection) != collection.GetSession() {
		BroadcastError(w, "Collection's session attribute is invalid", http.StatusBadRequest)
		return
	}

	// Creates a Job and put into the JobQueue for processing.
	for _, payload := range collection.Payloads {
		job := Job{dialects.NewEvent(collection, payload), 1}
		jobQueue <- &job
	}

	// Returns with 200.
	w.WriteHeader(http.StatusOK)
}

// Runs before the program starts
func main() {
	// Parse the CLI's attributes
	var filename = flag.String("config", "", "configuration `file` for the dialect")
	flag.BoolVar(&verbose, "verbose", false, "verbose mode for debugging")
	flag.Parse()

	if *filename == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Set a prefix for the logger
	log.SetPrefix(fmt.Sprintf("hamustro-%s ", Version))

	// Read and parse the configuration file
	config = NewConfig(*filename)
	if !config.IsValid() {
		log.Fatalf("Config is incomplete, please define `dialect` and `shared_secret` property")
	}
	dialect, err := config.DialectConfig()
	if err != nil {
		log.Fatalf("Loading dialect configuration is failed: %s", err.Error())
	}
	if !dialect.IsValid() {
		log.Fatalf("Dialect configuration is incorrect or incomplete: %s", err.Error())
	}

	// Construct the dialect's client
	storageClient, err = dialect.NewClient()
	if err != nil {
		log.Fatalf("Client initialization is failed: %s", err.Error())
	}

	// Create the background workers
	jobQueue = make(chan *Job, config.GetMaxQueueSize())
	dispatcher = NewDispatcher(config.GetMaxWorkerSize(), config.GetBufferSize(), config.IsSpreadBuffer())
	dispatcher.Run()

	// Capture SIGINT and SIGTERM events to finish the ongoing work
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt)
	signal.Notify(signalChannel, syscall.SIGTERM)
	go func() {
		<-signalChannel
		cleanup()
		os.Exit(1)
	}()

	// Set the log's output
	if config.LogFile != "" {
		logFile, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("Can't open logfile %s", err.Error())
		}
		defer logFile.Close()
		log.SetOutput(logFile)
	}

	// Start the server
	log.Printf("Starting server at %s", config.GetAddress())
	http.HandleFunc("/api/v1/track", TrackHandler)
	http.ListenAndServe(config.GetAddress(), nil)
}

// Runs after the server was shut down
func cleanup() {
	// Do not accept new requests
	isTerminating = true
	log.Println("Shutting down server ...")

	// Set a timeout interval to force stop (avoid hanging out)
	go func() {
		c := time.Tick(90 * time.Second)
		for range c {
			log.Fatal("Server shut down is taking too long, force quit immediately.")
		}
	}()

	// Try to stop every worker
	dispatcher.Stop()
}
