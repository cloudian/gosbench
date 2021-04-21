package common

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	log "github.com/sirupsen/logrus"
)

// This uses the Base 2 calculation where
// 1 kB = 1024 Byte
const (
	BYTE = 1 << (10 * iota)
	KILOBYTE
	MEGABYTE
	GIGABYTE
	TERABYTE
)

// S3Configuration contains all information to connect to a certain S3 endpoint
type S3Configuration struct {
	AccessKey     string        `yaml:"access_key" json:"access_key"`
	SecretKey     string        `yaml:"secret_key" json:"secret_key"`
	Region        string        `yaml:"region" json:"region"`
	Endpoint      string        `yaml:"endpoint" json:"endpoint"`
	Timeout       time.Duration `yaml:"timeout" json:"timeout"`
	SkipSSLVerify bool          `yaml:"skipSSLverify" json:"skipSSLverify"`
	ProxyHost     string        `yaml:"proxyHost" json:"proxyHost"`
}

// GrafanaConfiguration contains all information necessary to add annotations
// via the Grafana HTTP API
type GrafanaConfiguration struct {
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
	Endpoint string `yaml:"endpoint" json:"endpoint"`
}

// TestCaseConfiguration is the configuration of a performance test
type TestCaseConfiguration struct {
	Objects struct {
		SizeMin            uint64 `yaml:"size_min" json:"size_min"`
		SizeMax            uint64 `yaml:"size_max" json:"size_max"`
		SizeLast           uint64
		SizeDistribution   string `yaml:"size_distribution" json:"size_distribution"`
		NumberMin          uint64 `yaml:"number_min" json:"number_min"`
		NumberMax          uint64 `yaml:"number_max" json:"number_max"`
		NumberLast         uint64
		NumberDistribution string `yaml:"number_distribution" json:"number_distribution"`
		Unit               string `yaml:"unit" json:"unit"`
	} `yaml:"objects" json:"objects"`
	Buckets struct {
		NumberMin          uint64 `yaml:"number_min" json:"number_min"`
		NumberMax          uint64 `yaml:"number_max" json:"number_max"`
		NumberLast         uint64
		NumberDistribution string `yaml:"number_distribution" json:"number_distribution"`
	} `yaml:"buckets" json:"buckets"`
	Multipart struct {
		WriteMPUEnabled  bool   `yaml:"write_mpu_enabled" json:"write_mpu_enabled"`
		WritePartSize    uint64 `yaml:"write_part_size" json:"write_part_size"`
		WriteConcurrency int    `yaml:"write_concurrency" json:"write_concurrency"`
		WriteUnit        string `yaml:"write_unit" json:"write_unit"`
		ReadMPUEnabled   bool   `yaml:"read_mpu_enabled" json:"read_mpu_enabled"`
		ReadPartSize     uint64 `yaml:"read_part_size" json:"read_part_size"`
		ReadConcurrency  int    `yaml:"read_concurrency" json:"read_concurrency"`
		ReadUnit         string `yaml:"read_unit" json:"read_unit"`
	} `yaml:"multipart" json:"multipart"`
	Name                string   `yaml:"name" json:"name"`
	BucketPrefix        string   `yaml:"bucket_prefix" json:"bucket_prefix"`
	ObjectPrefix        string   `yaml:"object_prefix" json:"object_prefix"`
	Runtime             Duration `yaml:"stop_with_runtime" json:"stop_with_runtime"`
	OpsDeadline         uint64   `yaml:"stop_with_ops" json:"stop_with_ops"`
	Drivers             int      `yaml:"drivers" json:"drivers"`
	DriversShareBuckets bool     `yaml:"drivers_share_buckets" json:"drivers_share_buckets"`
	Workers             int      `yaml:"workers" json:"workers"`
	CleanAfter          bool     `yaml:"clean_after" json:"clean_after"`
	ReadWeight          int      `yaml:"read_weight" json:"read_weight"`
	ExistingReadWeight  int      `yaml:"existing_read_weight" json:"existing_read_weight"`
	WriteWeight         int      `yaml:"write_weight" json:"write_weight"`
	ListWeight          int      `yaml:"list_weight" json:"list_weight"`
	DeleteWeight        int      `yaml:"delete_weight" json:"delete_weight"`
}

// Workloadconf the Grafana and test configuration
type Workloadconf struct {
	GrafanaConfig *GrafanaConfiguration    `yaml:"grafana_config" json:"grafana_config"`
	Tests         []*TestCaseConfiguration `yaml:"tests" json:"tests"`
}

// Testconf contains all the information necessary to set up a distributed test
type Testconf struct {
	S3Config      []*S3Configuration       `yaml:"s3_config" json:"s3_config"`
	GrafanaConfig *GrafanaConfiguration    `yaml:"grafana_config" json:"grafana_config"`
	Tests         []*TestCaseConfiguration `yaml:"tests" json:"tests"`
}

// DriverConf is the configuration that is sent to each driver
// It includes a subset of information from the Testconf
type DriverConf struct {
	S3Config *S3Configuration
	Test     *TestCaseConfiguration
	DriverID string
}

// BenchResult is the struct that will contain the benchmark results from a
// driver after it has finished its benchmark
type BenchmarkResult struct {
	Host             string
	TestName         string
	OperationName    string
	ObjectSize       float64
	Operations       float64
	FailedOperations float64
	OpsPerSecond     float64
	Workers          int
	Bytes            float64
	// Bandwidth is the amount of Bytes per second of runtime
	Bandwidth    float64
	LatencyAvg   float64
	SuccessRatio float64
	StartTime    time.Time
	StopTime     time.Time
	Duration     time.Duration
	Options      string
}

// DriverMessage is the struct that is exchanged in the communication between
// server and driver. It usually only contains a message, but during the init
// phase, also contains the config for the driver
type DriverMessage struct {
	Message     string
	Config      *DriverConf
	BenchResult BenchmarkResult
}

// CheckConfig checks the global config
func CheckConfig(config Testconf) {
	for _, testcase := range config.Tests {
		// log.Debugf("Checking testcase with prefix %s", testcase.BucketPrefix)
		err := checkTestCase(testcase)
		if err != nil {
			log.WithError(err).Fatalf("Issue detected when scanning through the config file:")
		}
	}
}

func checkTestCase(testcase *TestCaseConfiguration) error {
	if testcase.Runtime == 0 && testcase.OpsDeadline == 0 {
		return fmt.Errorf("Either stop_with_runtime or stop_with_ops needs to be set")
	}
	if testcase.ReadWeight == 0 && testcase.WriteWeight == 0 && testcase.ListWeight == 0 && testcase.DeleteWeight == 0 && testcase.ExistingReadWeight == 0 {
		return fmt.Errorf("At least one weight needs to be set - Read / Write / List / Delete")
	}
	if testcase.ExistingReadWeight != 0 && testcase.BucketPrefix == "" {
		return fmt.Errorf("When using existing_read_weight, setting the bucket_prefix is mandatory")
	}
	if testcase.Buckets.NumberMin == 0 {
		return fmt.Errorf("Please set minimum number of Buckets")
	}
	if testcase.Objects.SizeMin == 0 {
		return fmt.Errorf("Please set minimum size of Objects")
	}
	if testcase.Objects.SizeMax == 0 {
		return fmt.Errorf("Please set maximum size of Objects")
	}
	if testcase.Objects.NumberMin == 0 {
		return fmt.Errorf("Please set minimum number of Objects")
	}
	if err := checkDistribution(testcase.Objects.SizeDistribution, "Object size_distribution"); err != nil {
		return err
	}
	if err := checkDistribution(testcase.Objects.NumberDistribution, "Object number_distribution"); err != nil {
		return err
	}
	if err := checkDistribution(testcase.Buckets.NumberDistribution, "Bucket number_distribution"); err != nil {
		return err
	}
	if testcase.Objects.Unit == "" {
		return fmt.Errorf("Please set the Objects unit")
	}

	toByteMultiplicator, err := getByteMultiplier(testcase.Objects.Unit)
	if err != nil {
		return err
	}
	testcase.Objects.SizeMin = testcase.Objects.SizeMin * toByteMultiplicator
	testcase.Objects.SizeMax = testcase.Objects.SizeMax * toByteMultiplicator

	toByteMultiplicator, err = getByteMultiplier(testcase.Multipart.WriteUnit)
	if err != nil {
		return err
	}
	testcase.Multipart.WritePartSize = testcase.Multipart.WritePartSize * toByteMultiplicator

	toByteMultiplicator, err = getByteMultiplier(testcase.Multipart.ReadUnit)
	if err != nil {
		return err
	}
	testcase.Multipart.ReadPartSize = testcase.Multipart.ReadPartSize * toByteMultiplicator
	return nil
}

// Checks if a given string is of type distribution
func checkDistribution(distribution string, keyname string) error {
	switch distribution {
	case "constant", "random", "sequential":
		return nil
	}
	return fmt.Errorf("%s is not a valid distribution. Allowed options are constant, random, sequential", keyname)
}

// EvaluateDistribution looks at the given distribution and returns a meaningful next number
func EvaluateDistribution(min uint64, max uint64, lastNumber *uint64, increment uint64, distribution string) uint64 {
	switch distribution {
	case "constant":
		return min
	case "random":
		rand.Seed(time.Now().UnixNano())
		validSize := max - min
		return ((rand.Uint64() % validSize) + min)
	case "sequential":
		if *lastNumber+increment > max {
			return max
		}
		*lastNumber = *lastNumber + increment
		return *lastNumber
	}
	return 0
}

func getByteMultiplier(unit string) (uint64, error) {
	switch strings.ToUpper(unit) {
	case "B":
		return BYTE, nil
	case "KB", "K":
		return KILOBYTE, nil
	case "MB", "M":
		return MEGABYTE, nil
	case "GB", "G":
		return GIGABYTE, nil
	case "TB", "T":
		return TERABYTE, nil
	default:
		return 0, fmt.Errorf("Could not parse unit size - please use one of B/KB/MB/GB/TB")
	}
}

type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = Duration(time.Duration(value))
		return nil
	case string:
		tmp, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = Duration(tmp)
		return nil
	default:
		return errors.New("invalid duration")
	}
}

func (d Duration) MarshalYAML() ([]byte, error) {
	return yaml.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var yamlDuration time.Duration
	err := unmarshal(yamlDuration)
	if err != nil {
		return err
	}

	*d = Duration(yamlDuration)
	return nil
}
