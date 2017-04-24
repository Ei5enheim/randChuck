package main

import (
    "net/http"
    "github.com/franela/goreq"
    "io/ioutil"
    "encoding/json"
    "fmt"
    "bytes"
    "time"
)

const (
    RandNameUri = "http://uinames.com/api/"
    RandJokeUri = "http://api.icndb.com/jokes/random?firstName=John&lastName=Doe&limitTo=[nerdy]"
)

/*
 * Stores job related info.
 */
type Job struct {
    resp http.ResponseWriter
    req *http.Request
}

type Worker interface {
    Start ()
    Stop ()
}

type ChuckWorker struct {
    fanoutFact int
    JobPool <-chan Job
    quit    chan bool
}

type NameResp struct {
    Name string `json:"name"`
    Surname string `json:"surname"`
    Gender string `json:"gender"`
    Region string `json:"region"`
}

type ChuckResp struct {
    Type string `json:"type"`
    Value  struct {
        Id json.Number `json:"id"`
        Joke string `json:"joke"`
        //Categories []string `json:"categories"`
    } `json:"value"`
}

func NewChuckWorker (jobPool <-chan Job, num int) Worker {
    return &ChuckWorker {
        fanoutFact: num,
        JobPool:    jobPool,
        quit:       make(chan bool, 1),
    }
}

func getUri (uri string, respChan chan<- []byte, errChan chan<- error) {

    fmt.Println("Im in")
    //<TODO> add time out and debug options as parameters
    req := goreq.Request{
            Uri:        uri,//http://uinames.com/api/,
            Timeout:    500 * time.Millisecond,
    }

    resp, err := req.Do()

    if err != nil {
        errChan <- err
        return
    }

    body, err := ioutil.ReadAll(resp.Body)

    if err != nil {
        errChan <- err
        return
    }

    respChan <- body
}

func fanoutGetUri (uri string, respChan chan<- []byte, errChan chan<- error, fanoutFact int) {

    fanoutRespChan := make (chan []byte, fanoutFact)
    fanoutErrChan  := make (chan error, fanoutFact)

    for i := 0; i < fanoutFact; i++ {
        go getUri(uri, fanoutRespChan, fanoutErrChan)
    }
    
    var randName []byte
    var err error
    count := 0

    for {
        select {
        case randName = <-fanoutRespChan:
            respChan <- randName
            return
        case err = <-fanoutErrChan:
            count++
        }

        if count == fanoutFact {
            errChan <- err
            return
        }
    }
}

func getRandNameJoke (respChan chan<- []byte, errChan chan<- error, fanoutFact int) {

    nameDataChan, jokeDataChan := make (chan []byte, 1), make (chan []byte, 1)
    nameErrChan, jokeErrChan := make (chan error, 1), make (chan error, 1)

    go fanoutGetUri(RandNameUri, nameDataChan, nameErrChan, fanoutFact)
    go fanoutGetUri(RandJokeUri, jokeDataChan, jokeErrChan, fanoutFact)
    
    count := 0
    var nameReqErr, jokeReqErr error
    var randNameBytes, randJokeBytes []byte

    for {
        select {
        case randNameBytes = <-nameDataChan:
            count++
        case randJokeBytes = <-jokeDataChan:
            count++
        case nameReqErr = <-nameErrChan:
            errChan <- nameReqErr
            return
        case jokeReqErr = <-jokeErrChan:
            errChan <- jokeReqErr
            return
        }

        if count == 2 {
            var randName NameResp
            var randJoke ChuckResp
            
            err := json.Unmarshal(randNameBytes, &randName)
            if err != nil {
                errChan <- err
                return
            }

            fmt.Println("name: ", string(randNameBytes))
            err = json.Unmarshal(randJokeBytes, &randJoke)
            if err != nil {
                errChan <- err
                return
            }
            
            fmt.Println("name: ", string(randNameBytes), string(randJokeBytes))
            var mixResp bytes.Buffer
            mixResp.WriteString(randName.Name)
            mixResp.WriteString(" ")
            mixResp.WriteString(randName.Surname)
            mixResp.WriteString(" ")
            mixResp.WriteString(randJoke.Value.Joke)
            
            respChan <- mixResp.Bytes()
        }
    }
}

func execNameChuckJob (job *Job, fanoutFact int) {

    respDataChan := make (chan []byte)
    errChan := make (chan error)

    go getRandNameJoke(respDataChan, errChan, fanoutFact)
    
    errCount := 0
    var err error

    for {
        select {
            case data := <-respDataChan:
                fmt.Println("string is ", string(data))
                job.resp.Header().Set("Content-Type", "application/json")
                job.resp.Write(data)
                return
            case err = <-errChan:
                errCount++
        }

        if errCount == fanoutFact {
            http.Error(job.resp, err.Error(), http.StatusInternalServerError) 
            fmt.Println(err)
            return
        }
    }
}

func (worker *ChuckWorker) Start () {

    for {
        select {
        case job := <- worker.JobPool:
            fmt.Println("Got a job")
            execNameChuckJob(&job, worker.fanoutFact)
        case quit := <- worker.quit: 
            fmt.Println("Need to get some rest.", quit)
            break;
        }
    }
}

func (worker *ChuckWorker) Stop () {
    worker.quit <- true
    return
}

/*
func main () {

    jobPool := make(chan Job)

    worker := NewChuckWorker(jobPool)
    fmt.Println("Starting a worker")
    go worker.Start()
    var j Job

    jobPool <- j 

    for {
        
    }
}*/
