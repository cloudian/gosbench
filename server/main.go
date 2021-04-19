package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"net"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mulbc/gosbench/common"
	"gopkg.in/yaml.v2"

	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	rand.Seed(time.Now().UnixNano())

	flag.StringVar(&configFileLocation, "c", "", "Config file describing test run")
	flag.StringVar(&s3FileLocation, "s", "", "S3 configuration information")
	flag.IntVar(&serverPort, "p", 2000, "Port on which the server will be available for clients. Default: 2000")
	flag.BoolVar(&debug, "d", false, "enable debug log output")
	flag.BoolVar(&trace, "t", false, "enable trace log output")
	flag.Parse()
	// Only demand this flag if we are not running go test
	if configFileLocation == "" && flag.Lookup("test.v") == nil {
		log.Fatal("-c is a mandatory parameter - please specify the config file")
	}
	if debug {
		log.SetLevel(log.DebugLevel)
	} else if trace {
		log.SetLevel(log.TraceLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
}

var configFileLocation string
var s3FileLocation string
var serverPort int
var readyDrivers chan *net.Conn
var done bool = false
var debug, trace bool
var listener net.Listener

func loadS3ConfigFromFile(s3FileContent []byte) []*common.S3Configuration {
	var s3Config []*common.S3Configuration
	var err error

	if strings.HasSuffix(s3FileLocation, ".yaml") {
		err = yaml.Unmarshal(s3FileContent, &s3Config)
		if err != nil {
			log.WithError(err).Fatalf("Error unmarshaling yaml s3 config file:")
		}
	} else if strings.HasSuffix(s3FileLocation, ".json") {
		err = json.Unmarshal(s3FileContent, &s3Config)
		if err != nil {
			log.WithError(err).Fatalf("Error unmarshaling json S3 config file:")
		}
	} else {
		log.WithError(err).Fatalf("S3 configuration file must be a yaml or json formatted file")
	}
	return s3Config
}

func loadConfigFromFile(configFileContent []byte) common.Workloadconf {
	var workload common.Workloadconf
	var err error

	if strings.HasSuffix(configFileLocation, ".yaml") {
		err = yaml.Unmarshal(configFileContent, &workload)
		if err != nil {
			log.WithError(err).Fatalf("Error unmarshaling yaml config file:")
		}
	} else if strings.HasSuffix(configFileLocation, ".json") {
		err = json.Unmarshal(configFileContent, &workload)
		if err != nil {
			log.WithError(err).Fatalf("Error unmarshaling json config file:")
		}
	} else {
		log.WithError(err).Fatalf("Configuration file must be a yaml or json formatted file")
	}
	return workload
}

func main() {

	configFileContent, err := ioutil.ReadFile(configFileLocation)
	if err != nil {
		log.WithError(err).Fatalf("Error reading workload config file:")
	}

	workload := loadConfigFromFile(configFileContent)

	s3FileContent, err := ioutil.ReadFile(s3FileLocation)
	if err != nil {
		log.WithError(err).Fatalf("Error reading S3vconfig file:")
	}

	s3Config := loadS3ConfigFromFile(s3FileContent)

	config := common.Testconf{
		S3Config:      s3Config,
		GrafanaConfig: workload.GrafanaConfig,
		Tests:         workload.Tests,
	}

	common.CheckConfig(config)

	readyDrivers = make(chan *net.Conn)
	defer close(readyDrivers)

	// Listen on TCP port 2000 on all available unicast and
	// anycast IP addresses of the local system.
	listener, err = net.Listen("tcp", fmt.Sprintf(":%d", serverPort))
	if err != nil {
		log.WithError(err).Fatal("Could not open port!")
	}
	log.Info("Ready to accept connections")
	go scheduleTests(config)
	for {
		// Wait for a connection.
		conn, err := listener.Accept()
		if done {
			break
		}
		if err != nil {
			log.WithError(err).Fatal("Issue when waiting for connection of clients")
		}
		// Handle the connection in a new goroutine.
		// The loop then returns to accepting, so that
		// multiple connections may be served concurrently.
		go func(c *net.Conn) {
			log.Infof("%s connected to us ", (*c).RemoteAddr())
			decoder := json.NewDecoder(*c)
			var message string
			err := decoder.Decode(&message)
			if err != nil {
				log.WithField("message", message).WithError(err).Error("Could not decode message, closing connection")
				(*c).Close()
				return
			}
			if message == "ready for work" {
				log.Debug("We have a new driver!")
				readyDrivers <- c
				return
			}
		}(&conn)

	}

	log.Infof("Shutting down server")
}

func scheduleTests(config common.Testconf) {

	var maxDrivers int = 0

	for testNumber, test := range config.Tests {

		doneChannel := make(chan bool, test.Drivers)
		resultChannel := make(chan common.BenchmarkResult, test.Drivers)
		continueDrivers := make(chan bool, test.Drivers)

		maxDrivers = int(math.Max(float64(test.Drivers), float64(maxDrivers)))

		for driver := 0; driver < test.Drivers; driver++ {
			driverConfig := &common.DriverConf{
				Test:     test,
				S3Config: config.S3Config[driver%len(config.S3Config)],
				DriverID: fmt.Sprintf("d%d", driver),
			}
			driverConnection := <-readyDrivers
			log.WithField("Driver", (*driverConnection).RemoteAddr()).Infof("We found driver %d / %d for test %d", driver+1, test.Drivers, testNumber)
			go executeTestOnDriver(driverConnection, driverConfig, doneChannel, continueDrivers, resultChannel)
		}
		for driver := 0; driver < test.Drivers; driver++ {
			// Will halt until all driverss are done with preparations
			<-doneChannel
		}
		// Add sleep after prep phase so that drives can relax
		time.Sleep(5 * time.Second)
		log.WithField("test", test.Name).Info("All drivers have finished preparations - starting performance test")
		startTime := time.Now().UTC()
		for driver := 0; driver < test.Drivers; driver++ {
			continueDrivers <- true
		}
		var benchResults []common.BenchmarkResult
		for driver := 0; driver < test.Drivers; driver++ {
			// Will halt until all drivers are done with their work
			<-doneChannel
			benchResults = append(benchResults, <-resultChannel)
		}
		stopTime := time.Now().UTC()
		log.WithField("test", test.Name).Info("All drivers have finished the performance test - continuing with next test")
		log.WithField("test", test.Name).Infof("GRAFANA: ?from=%d&to=%d", startTime.UnixNano()/int64(1000000), stopTime.UnixNano()/int64(1000000))
		benchResult := sumBenchmarkResults(benchResults)
		benchResult.StartTime = startTime
		benchResult.StopTime = stopTime
		benchResult.Duration = stopTime.Sub(startTime)
		log.WithField("test", test.Name).
			WithField("Operation Name", benchResult.OperationName).
			WithField("Drivers", benchResult.Workers).
			WithField("Object Size", benchResult.ObjectSize).
			WithField("Completed Operations", benchResult.Operations).
			WithField("Failed Operations", benchResult.FailedOperations).
			WithField("Ops Per Second", benchResult.OpsPerSecond).
			WithField("Total Bytes", benchResult.Bytes).
			WithField("Average BW in Byte/s", benchResult.Bandwidth).
			WithField("Average latency in ms", benchResult.LatencyAvg).
			WithField("Success Ratio", benchResult.SuccessRatio).
			WithField("Start Time", benchResult.StartTime).
			WithField("Stop Time", benchResult.StopTime).
			WithField("Test runtime on server", benchResult.Duration).
			Infof("PERF RESULTS")
		writeResultToCSV(benchResult)
		writeResultToConsole(benchResults, benchResult)
		close(doneChannel)
		close(continueDrivers)
		close(resultChannel)
	}
	log.Info("All performance tests finished")
	for driver := 0; driver < maxDrivers; driver++ {
		driverConnection := <-readyDrivers
		shutdownDriver(driverConnection)
	}
	done = true
	listener.Close()
}

func executeTestOnDriver(conn *net.Conn, config *common.DriverConf, doneChannel chan bool, continueDrivers chan bool, resultChannel chan common.BenchmarkResult) {
	encoder := json.NewEncoder(*conn)
	decoder := json.NewDecoder(*conn)
	_ = encoder.Encode(common.DriverMessage{Message: "init", Config: config})

	var response common.DriverMessage
	for {
		err := decoder.Decode(&response)
		if err != nil {
			log.WithField("driver", config.DriverID).WithField("message", response).WithError(err).Error("Driver responded unusually - dropping")
			(*conn).Close()
			return
		}
		log.Tracef("Response: %+v", response)
		switch response.Message {
		case "preparations done":
			doneChannel <- true
			<-continueDrivers
			_ = encoder.Encode(common.DriverMessage{Message: "start work"})
		case "work done":
			doneChannel <- true
			resultChannel <- response.BenchResult
			(*conn).Close()
			return
		}
	}
}

func shutdownDriver(conn *net.Conn) {
	encoder := json.NewEncoder(*conn)
	log.WithField("Driver", (*conn).RemoteAddr()).Info("Shutting down driver")
	_ = encoder.Encode(common.DriverMessage{Message: "shutdown"})
}

func sumBenchmarkResults(results []common.BenchmarkResult) common.BenchmarkResult {
	sum := common.BenchmarkResult{}
	bandwidthAverages := float64(0)
	latencyAverages := float64(0)
	objectSizeAverages := float64(0)
	for _, result := range results {
		sum.Bytes += result.Bytes
		sum.Operations += result.Operations
		sum.FailedOperations += result.FailedOperations
		latencyAverages += result.LatencyAvg
		bandwidthAverages += result.Bandwidth
		objectSizeAverages += result.ObjectSize
		sum.OpsPerSecond += result.OpsPerSecond
		sum.Workers += result.Workers
	}
	sum.SuccessRatio = sum.Operations / (sum.Operations + sum.FailedOperations)
	sum.LatencyAvg = latencyAverages / float64(len(results))
	sum.ObjectSize = objectSizeAverages / float64(len(results))
	sum.TestName = results[0].TestName
	sum.OperationName = results[0].OperationName
	sum.Options = results[0].Options
	sum.Bandwidth = bandwidthAverages
	return sum
}

func writeResultToCSV(benchResult common.BenchmarkResult) {
	file, created, err := getCSVFileHandle()
	if err != nil {
		log.WithError(err).Error("Could not get a file handle for the CSV results")
		return
	}
	defer file.Close()

	csvwriter := csv.NewWriter(file)

	if created {
		err = csvwriter.Write([]string{
			"TestName",
			"Operation Name",
			"Workers",
			"Object Size",
			"Completed Operations",
			"Failed Operations",
			"Ops/Second",
			"Total Bytes",
			"Bandwidth in Bytes/s",
			"Average Latency in ms",
			"Success Ratio",
			"Start Time",
			"Stop Time",
			"Test duration seen by server in seconds",
			"Test Options",
		})
		if err != nil {
			log.WithError(err).Error("Failed writing line to results csv")
			return
		}
	}

	err = csvwriter.Write([]string{
		benchResult.TestName,
		benchResult.OperationName,
		fmt.Sprintf("%d", benchResult.Workers),
		fmt.Sprintf("%.0f", benchResult.ObjectSize),
		fmt.Sprintf("%.0f", benchResult.Operations),
		fmt.Sprintf("%.0f", benchResult.FailedOperations),
		fmt.Sprintf("%f", benchResult.OpsPerSecond),
		fmt.Sprintf("%.0f", benchResult.Bytes),
		fmt.Sprintf("%f", benchResult.Bandwidth),
		fmt.Sprintf("%f", benchResult.LatencyAvg),
		fmt.Sprintf("%.2f", benchResult.SuccessRatio),
		fmt.Sprintf("%d", benchResult.StartTime.Unix()),
		fmt.Sprintf("%d", benchResult.StopTime.Unix()),
		fmt.Sprintf("%f", benchResult.Duration.Seconds()),
		benchResult.Options,
	})
	if err != nil {
		log.WithError(err).Error("Failed writing line to results csv")
		return
	}

	csvwriter.Flush()

}

func getCSVFileHandle() (*os.File, bool, error) {
	file, err := os.OpenFile("gosbench_results.csv", os.O_APPEND|os.O_WRONLY, 0755)
	if err == nil {
		return file, false, nil
	}
	file, err = os.OpenFile("/tmp/gosbench_results.csv", os.O_APPEND|os.O_WRONLY, 0755)
	if err == nil {
		return file, false, nil
	}

	file, err = os.OpenFile("gosbench_results.csv", os.O_WRONLY|os.O_CREATE, 0755)
	if err == nil {
		return file, true, nil
	}
	file, err = os.OpenFile("/tmp/gosbench_results.csv", os.O_WRONLY|os.O_CREATE, 0755)
	if err == nil {
		return file, true, nil
	}

	return nil, false, errors.New("Could not find previous CSV for appending and could not write new CSV file to current dir and /tmp/ giving up")

}

func writeResultToConsole(driverResult []common.BenchmarkResult, summedResults common.BenchmarkResult) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "HOST\tTEST NAME\tOP NAME\tWORKERS\tOBJECT SIZE\tCOMPLETED OPS\tFAILED OPS\tOPS PER SECOND\tTOTAL MB\tBANDWIDTH (MB)\tLATENCY\tSUCCESS RATIO\tDURATION\t")
	for _, result := range driverResult {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%.0f\t%.0f\t%.0f\t%.2f ops/sec\t%.2f MB\t%.2f MB/s\t%.2f ms\t%.2f%%\t%.2f s\t\n",
			result.Host, result.TestName, result.OperationName, result.Workers, result.ObjectSize, result.Operations,
			result.FailedOperations, result.OpsPerSecond, result.Bytes/(1024*1024), result.Bandwidth/(1024*1024),
			result.LatencyAvg, result.SuccessRatio*100, result.Duration.Seconds())
	}
	fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%.0f\t%.0f\t%.0f\t%.2f ops/sec\t%.2f MB\t%.2f MB/s\t%.2f ms\t%.2f%%\t%.2f s\t\n",
		"Totals", summedResults.TestName, summedResults.OperationName, summedResults.Workers, summedResults.ObjectSize,
		summedResults.Operations, summedResults.FailedOperations, summedResults.OpsPerSecond, summedResults.Bytes/(1024*1024),
		summedResults.Bandwidth/(1024*1024), summedResults.LatencyAvg, summedResults.SuccessRatio*100, summedResults.Duration.Seconds())

	w.Flush()
}
