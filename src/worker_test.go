package main

import (
	"./dialects"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"sync"
	"testing"
)

// Global variables for testing (hacky)
var T *testing.T
var exp *bytes.Buffer
var sResp error = nil

// Returns an Event for testing purposes
func GetTestEvent(userId uint32) *dialects.Event {
	return &dialects.Event{
		DeviceID:       "a73b1c37-2c24-4786-af7a-16de88fbe23a",
		ClientID:       "bce44f67b2661fd445d469b525b04f68",
		Session:        "244f056dee6d475ec673ea0d20b69bab",
		Nr:             1,
		SystemVersion:  "10.10",
		ProductVersion: "1.1.2",
		At:             "2016-02-05T15:05:04",
		Event:          "Client.CreateUser",
		System:         "OSX",
		ProductGitHash: "5416a5889392d509e3bafcf40f6388e83aab23e6",
		UserID:         userId,
		IP:             "214.160.227.22",
		Parameters:     "",
		IsTesting:      false}
}

// Simple (not buffered) Storage Client for testing
type SimpleStorageClient struct{}

func (c *SimpleStorageClient) IsBufferedStorage() bool {
	return false
}
func (c *SimpleStorageClient) GetConverter() dialects.Converter {
	return dialects.ConvertJSON
}
func (c *SimpleStorageClient) GetBatchConverter() dialects.BatchConverter {
	return nil
}
func (c *SimpleStorageClient) Save(msg *bytes.Buffer) error {
	if sResp != nil {
		return sResp
	}
	T.Log("Validating received message within the SimpleStorageClient")
	if exp.String() != msg.String() {
		T.Errorf("Expected message was `%s` and it was `%s` instead.", exp, msg)
	}
	return nil
}

// Tests the simple storage client (not buffered) with a single worker
func TestSimpleStorageClientWorker(t *testing.T) {
	// Disable the logger
	log.SetOutput(ioutil.Discard)

	// Define the job Queue and the Simple Storage Client
	jobQueue = make(chan *Job, 10)
	storageClient = &SimpleStorageClient{}

	// Make testing.T and the response global
	T = t
	sResp = nil

	// Create a worker
	t.Log("Creating a single worker")
	pool := make(chan chan *Job, 2)
	worker := NewWorker(1, 10, pool)
	worker.RetryAttempt = 2
	worker.Start()

	// Stop the worker on the end
	var wg sync.WaitGroup
	wg.Add(1)
	defer worker.Stop(&wg)

	// Start the test
	jobChannel := <-pool

	t.Log("Creating a single job and send it to the worker")
	job := Job{GetTestEvent(3423543), 1}
	exp, _ = dialects.ConvertJSON(job.Event)
	jobChannel <- &job
	jobChannel = <-pool

	t.Log("Creating an another single job and send it to the worker")
	job = Job{GetTestEvent(1321), 1}
	exp, _ = dialects.ConvertJSON(job.Event)
	jobChannel <- &job
	jobChannel = <-pool

	t.Log("Send something that will fail and raise an error")
	sResp = fmt.Errorf("Error was intialized for testing")
	job = Job{GetTestEvent(43233), 1}
	exp, _ = dialects.ConvertJSON(job.Event)
	jobChannel <- &job
	jobChannel = <-pool

	if job.Attempt != 2 {
		t.Errorf("Job attempt number should be %d and it was %d instead", 2, job.Attempt)
	}

	t.Log("This failed message must be in the jobQueue, try again.")
	if len(jobQueue) != 1 {
		t.Errorf("jobChannel doesn't contain the previous job")
	}
	jobq := <-jobQueue
	sResp = nil
	jobChannel <- jobq
	jobChannel = <-pool

	t.Log("Send something that will fail and raise an error again")
	sResp = fmt.Errorf("Error was intialized for testing")
	job = Job{GetTestEvent(43254534), 1}
	exp, _ = dialects.ConvertJSON(job.Event)
	jobChannel <- &job
	jobChannel = <-pool

	t.Log("This failed message must be in the jobQueue, but let it fail again.")
	if len(jobQueue) != 1 {
		t.Errorf("jobQueue doesn't contain the previous job")
	}
	jobq = <-jobQueue
	jobChannel <- jobq
	jobChannel = <-pool

	if job.Attempt != 3 {
		t.Errorf("Job attempt number should be %d and it was %d instead", 3, job.Attempt)
	}
	if len(jobQueue) != 0 {
		t.Errorf("jobQueue have to be empty because it was dropped after the 2nd attempt")
	}
}

// Worker Id testing
func TestGetId(t *testing.T) {
	jobQueue = make(chan *Job, 10)
	storageClient = &SimpleStorageClient{}

	t.Log("Creating a worker with 312 id")
	pool := make(chan chan *Job, 1)
	worker := NewWorker(312, 10, pool)

	if worker.GetId() != 312 {
		t.Errorf("Expected worker's ID was %d but it was %d instead.", 312, worker.GetId())
	}
}

// Buffered Storage Client for testing
type BufferedStorageClient struct{}

func (c *BufferedStorageClient) IsBufferedStorage() bool {
	return true
}
func (c *BufferedStorageClient) GetConverter() dialects.Converter {
	return nil
}
func (c *BufferedStorageClient) GetBatchConverter() dialects.BatchConverter {
	return dialects.ConvertBatchJSON
}
func (c *BufferedStorageClient) Save(msg *bytes.Buffer) error {
	if sResp != nil {
		return sResp
	}
	T.Log("Validating received messages within the BufferedStorageClient")
	if exp.String() != msg.String() {
		T.Errorf("Expected message was `%s` and it was `%s` instead.", exp, msg)
	}
	return nil
}

// Tests the simple storage client (not buffered) with a single worker
func TestBufferedStorageClientWorker(t *testing.T) {
	// Disable the logger
	log.SetOutput(ioutil.Discard)

	// Define the job Queue and the Buffered Storage Client
	jobQueue = make(chan *Job, 10)
	storageClient = &BufferedStorageClient{}

	// Make testing.T and the response global
	T = t
	sResp = nil

	// Create a worker
	t.Log("Creating a single worker")
	pool := make(chan chan *Job, 2)
	worker := NewWorker(1, 10, pool)
	worker.Start()

	// Stop the worker on the end
	var wg sync.WaitGroup
	wg.Add(1)
	defer worker.Stop(&wg)

	// Start the test
	jobChannel := <-pool
	var job Job

	t.Log("Creating 9 job and send it to the worker")
	partStr := ""
	for i := 0; i < 9; i++ {
		job = Job{GetTestEvent(uint32(56746535 + i)), 1}
		part, _ := dialects.ConvertJSON(job.Event)
		partStr += part.String()
		jobChannel <- &job
		jobChannel = <-pool

		if exnr := i + 1; len(worker.BufferedEvents) != exnr {
			t.Errorf("Worker's buffered events count should be %d but it was %d instead", exnr, len(worker.BufferedEvents))
		}
	}

	t.Log("Creating the 10th job and send it to the worker that will proceed the buffer")
	job = Job{GetTestEvent(1), 1}
	part, _ := dialects.ConvertJSON(job.Event)
	partStr += part.String()
	exp = bytes.NewBuffer([]byte(partStr))
	jobChannel <- &job
	jobChannel = <-pool

	if exnr := 0; len(worker.BufferedEvents) != exnr {
		t.Errorf("Worker's buffered events count should be %d but it was %d instead", exnr, len(worker.BufferedEvents))
	}
	if expen := float32(1.0); worker.Penalty != expen {
		t.Errorf("Expected worker's penalty was %d but it was %d instead", expen, worker.Penalty)
	}
	if exnr := 10; worker.GetBufferSize() != exnr {
		t.Errorf("Expected worker's buffer size after the error was %d but it was %d instead", exnr, worker.GetBufferSize())
	}

	t.Log("Creating 14 job and send it to the worker, during the process it'll fail after the 10th")
	sResp = fmt.Errorf("Error was intialized for testing")
	partStr = ""
	for i := 0; i < 14; i++ {
		job = Job{GetTestEvent(uint32(213432 + i)), 1}
		part, _ := dialects.ConvertJSON(job.Event)
		partStr += part.String()
		jobChannel <- &job
		jobChannel = <-pool

		if exnr := i + 1; len(worker.BufferedEvents) != exnr {
			t.Errorf("Worker's buffered events count should be %d but it was %d instead", exnr, len(worker.BufferedEvents))
		}
	}

	if expen := float32(1.5); worker.Penalty != expen {
		t.Errorf("Expected worker's penalty was %d but it was %d instead", expen, worker.Penalty)
	}
	if exnr := 15; worker.GetBufferSize() != exnr {
		t.Errorf("Expected worker's buffer size after the error was %d but it was %d instead", exnr, worker.GetBufferSize())
	}

	sResp = nil
	t.Log("Creating the 15th job and send it to the worker that will proceed the buffer")
	job = Job{GetTestEvent(1), 1}
	part, _ = dialects.ConvertJSON(job.Event)
	partStr += part.String()
	exp = bytes.NewBuffer([]byte(partStr))
	jobChannel <- &job
	jobChannel = <-pool

	if exnr := 0; len(worker.BufferedEvents) != exnr {
		t.Errorf("Worker's buffered events count should be %d but it was %d instead", exnr, len(worker.BufferedEvents))
	}
	if expen := float32(1.0); worker.Penalty != expen {
		t.Errorf("Expected worker's penalty was %d but it was %d instead", expen, worker.Penalty)
	}
	if exnr := 10; worker.GetBufferSize() != exnr {
		t.Errorf("Expected worker's buffer size after the error was %d but it was %d instead", exnr, worker.GetBufferSize())
	}
}
