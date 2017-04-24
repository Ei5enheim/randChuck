package main

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "os"
	"github.com/urfave/negroni"
	"github.com/thoas/stats"
    "net/http"
    "strconv"
    //"github.com/uber-go/zap"
)

type Config struct {
    FanoutFact int `json:"fanoutFact"`
    LogLevel string `json:"logLevel"`
    NoOfWorkers int `json:"Workers"`
    Port int `json:"port"`
}

func readConfig(configFile string) (con Config, err error) {
    
    content, err := ioutil.ReadFile(configFile)

    if err != nil {
        fmt.Println("Error " + err.Error()+", Unable to read the config file: "+ configFile)
        return 
    }

    err = json.Unmarshal(content, &con)

    if err != nil {
        fmt.Println("Error " + err.Error()+", Unable to parse the content in the config file.")
        return
    }

    return
}


func startWorkers(jobPool chan Job, nWorkers, fanoutFact int) []Worker {
    
    workers := make([]Worker, nWorkers)

    for i := 0; i < nWorkers; i++ {
        workers[i] = NewChuckWorker(jobPool, fanoutFact)
        go workers[i].Start()
    }

    return workers
}

func startServer (port int, jobPool chan Job) {

    mux := http.NewServeMux()
    statsMw := stats.New()

    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        j := Job {
                    req: r,
                    resp: w,
                }
                jobPool <- j
            })

    mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")

        stats := statsMw.Data()

        b, _ := json.Marshal(stats)

        w.Write(b)
    })
	
	n := negroni.New()
	n.Use(statsMw)
	n.UseHandler(mux)
    fmt.Println(":"+ strconv.Itoa(port))
    n.Run(":"+ strconv.Itoa(port))
}

func setupServer(c Config) {

    jobPool := make(chan Job, c.NoOfWorkers)

    startWorkers(jobPool, c.NoOfWorkers, c.FanoutFact)
    
    startServer(c.Port, jobPool)

}

func main() {

    if len(os.Args) < 2 {
        fmt.Println("Please provide the path to the config file as a command line argument")
        return
    }

    config, err := readConfig(os.Args[1])
    
    if err != nil {
        fmt.Println("Exiting")
        return
    }

    fmt.Println("config is ", config)

    setupServer(config)
}
