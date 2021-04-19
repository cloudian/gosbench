## S3 Configuration

The S3 configuration section allows for a list of S3 servers to be configured for testing. If load balancer will be used, you may only need a single S3 configuration. If no load balancer is available you can specify multiple individual S3 servers, and Gosbench will assign the servers out evenly. 

### Configuration Options:
- **access_key** - Access key for S3 credentials
- **secret_key** - Secret key for S3 credentials
- **region** - Region to use for testing
- **endpoint** - The full HTTP(S) URL to use for S3 request. This URl should include a port if needed. Example: https://my.rgw.endpoint:8080
- **skipSSLverify** - Should be set to true or false. True does not enforce strict validation of server certificate, false does enforce strict validation.
- **proxyHost** - The full HTTP(S) URL to use for proxy request. This URl should include a port if needed. Example: http://localhost:1234

## Grafana Configuration

The Grafana configuration is used by Gosbench to send annotations to the Grafana DB when test jobs start and stop.

### Configuration Options:
- **endpoint** - The full HTTP(S) URL to the Grafana server http://grafana:3000
- **username** - Grafana admin username
- **password** - Password for username

## Test Configuration
The test configuration specifies the details of the test to be performed, including which operations to run, bucket/object names, object size, etc. The test configuration section has several top level parameters as well as parameters that contain subsections, such as “objects”, “buckets” and “multipart”.

### Configuration Options:
Top Level Options:
- **name** - Name of the test
- **read_weight** - The priority to give to read requests
- **existing_read_weight** - The priority to give to existing_read requests
- **write_weight** - The priority to give to existing_read requests
- **delete_weight** - The priority to give to existing_read requests
- **list_weight** - The priority to give to existing_read requests
- **bucket_prefix** - String to use as  a prefix for bucket names
- **object_prefix** - String to use as  a prefix for bucket names
- **stop_with_runtime** - If this option is set to any value greater than 0 the test will run for the specified amount of time, then it will stop. The “stop_with_runtime” takes precedence over the “stop_with_ops” parameter. If both are set, only the “stop_with_runtime” will be used. Be sure that a unit suffix is provided, such as “60s”, "300m", "1.5h" or "2h45m". Valid time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".
- **stop_with_ops** - Specifies the number of operations to run before ending the test.
- **drivers** - The number of drivers that the server should expect to connect before starting the tests
- **workers** - The number of workers (or threads) that each driver should start up to run S3 commands
- **workers_share_buckets** -  If true, all workers will use the same buckers to read, write, lisy, and delete objects from.
- **clean_after** - If true, Gosbench will delete all buckets and objects created during the test until number max is reached, then only number)max will be used.

### Objects Options:
- **size_min** - Minimum size of object to use
- **size_max** - Maximum size object to use
- **size_distribution** - This parameter defines how object sizes are distributed. The valid values for this parameter are “constant”, “random”, “sequential”. If “constant” is set then only the size_min value is used for the object size. If “random” is set, then any value >= size_min and <= size_max may be used. If “sequential” is set the object size will start at size_min and the size will increment by 1 on each test.
- **unit** - The unit to use for size_min and size_max. Valid values are: B, K or KB, M or MB, G or GB, and T or TB. Either upper or lower case characters can be used.
- **number_min** - The minimum number value to use when generating a number suffix for object names.
- **number_max** - The maximum number value to use when generating a number suffix for object names.
- **number_distribution** - This parameter defines how object numbers are distributed. The valid values for this parameter are “constant”, “random”, “sequential”. If “constant” is set then only the number_min value is used for the object size. If “random” is set, then any value >= number_min and <= number_max may be used. If “sequential” is set the object size will start at number_min and the size will increment by 1 on each test until number max is reached, then only number)max will be used.

### Buckets Options:
- **number_min** - The minimum number value to use when generating a number suffix for bucket names.
- **number_max** - The maximum number value to use when generating a number suffix for bucket names.
- **number_distribution** - This parameter defines how object numbers are distributed. The valid values for this parameter are “constant”, “random”, “sequential”. If “constant” is set then only the number_min value is used for the object size. If “random” is set, then any value >= number_min and <= number_max may be used. If “sequential” is set the object size will start at number_min and the size will increment by 1 on each test until number max is reached, then only number)max will be used.

### Multipart Options:
- **write_mpu_enabled** - If true, this enables multipart writes using AWS’s upload manager. False, will use the putObject() function for uploading objects in a single request
- **write_part_size** - Specifies the size each part should be for multipart requests
- **write_unit** - The unit to use for write_part_size. Valid values are: B, K or KB, M or MB, G or GB, and T or TB. Either upper or lower case characters can be used.
- **write_concurrency** - The number of threads used by the upload manager to send parts simultaneously.
- **read_mpu_enabled** - If true, this enables multipart reads using AWS’s down manager. False, will use the getObject() function for downloading objects in a single request
- **read_part_size** - Specifies the size each part should be for multipart requests
- **read_unit** - The unit to use for read_part_size. Valid values are: B, K or KB, M or MB, G or GB, and T or TB. Either upper or lower case characters can be used.
read_concurrency - The number of threads used by the download manager to receive parts simultaneously.

## JSON Example Coniguration 
### S3 Configuration
```json

[
  { 
    "access_key": "abc", "secret_key": "as", 
    "region": "eu-central-1", "endpoint": "https://my.rgw.endpoint:8080", 
    "skipSSLverify": false, "proxyHost": "http://localhost:1234" 
  },
  { 
    "access_key": "def", "secret_key": "as", 
    "region": "eu-central-2", "endpoint": "https://my.rgw.endpoint:8080", 
    "skipSSLverify": false, "proxyHost": "http://localhost:1234" 
  },
  { 
    "access_key": "ghi", "secret_key": "as", 
    "region": "eu-central-3", "endpoint": "https://my.rgw.endpoint:8080", 
    "skipSSLverify": false, "proxyHost": "http://localhost:1234" 
  }
]

```

### Test Coniguration
```json
{
    "grafana_config": { "endpoint": "http://grafana", "username": "admin", "password": "grafana" },
    "tests": [
        { "name": "My first example test", "read_weight": 20, "existing_read_weight": 0, "write_weight": 80, "delete_weight": 0, 
          "list_weight": 0, "bucket_prefix": "1255gosbench-", "object_prefix": "obj", "stop_with_runtime": "1h30m", "stop_with_ops": 10, 
          "drivers": 6, "workers_share_buckets": true, "workers": 30, "clean_after": true,
          "objects": {"size_min": 5, "size_max": 100, "size_distribution": "random", "unit": "KB", 
                      "number_min": 10, "number_max": 10, "number_distribution": "constant" },
          "buckets": { "number_min": 1, "number_max": 10, "number_distribution": "constant" },
          "multipart": { "write_mpu_enabled": true, "write_part_size": 5, "write_unit": "MB", "write_concurrency": 5,
                         "read_mpu_enabled": true, "read_part_size": 5, "read_unit": "MB", "read_concurrency": 5 }
        }
    ]
}
```

## YAML Example Coniguration 
### S3 Configuration
```yaml
---

- access_key: abc
  secret_key: as
  region: eu-central-1
  endpoint: https://my.rgw.endpoint:8080
  skipSSLverify: false
  proxyHost: http://localhost:1234"
- access_key: def
  secret_key: as
  region: eu-central-2
  endpoint: https://my.rgw.endpoint:8080
  skipSSLverify: false
  proxyHost: http://localhost:1234"
- access_key: ghi
  secret_key: as
  region: eu-central-3
  endpoint: https://my.rgw.endpoint:8080
  skipSSLverify: false
  proxyHost: http://localhost:1234"

...
```

### Test Coniguration
```yaml
# For generating annotations when we start/stop test cases
# https://grafana.com/docs/http_api/annotations/#create-annotation
grafana_config:
  endpoint: http://grafana
  username: admin
  password: grafana

tests:
  - name: My first example test
    read_weight: 20
    existing_read_weight: 0
    write_weight: 80
    delete_weight: 0
    list_weight: 0
    objects:
      size_min: 5
      size_max: 100
      # distribution: constant, random, sequential
      size_distribution: random
      unit: KB
      number_min: 10
      number_max: 10
      # distribution: constant, random, sequential
      number_distribution: constant
    buckets:
      number_min: 1
      number_max: 10
      # distribution: constant, random, sequential
      number_distribution: constant
    multipart:
      write_mpu_enabled: true
      write_part_size: 5
      write_unit: MB
      write_concurrency: 5
      read_mpu_enabled: true
      read_part_size: 5
      read_unit: MB
      read_concurrency: 5
    # Name prefix for buckets and objects
    bucket_prefix: 1255gosbench-
    object_prefix: obj
    # End after a set amount of time
    # Runtime in time.Duration - do not forget the unit please
    # stop_with_runtime: 60s # Example with 60 seconds runtime
    stop_with_runtime:
    # End after a set amount of operations (per driver)
    stop_with_ops: 10
    # Number of s3 performance test servers to run in parallel
    drivers: 2
    # Set whether drivers share the same buckets or not
    # If set to True - bucket names will have the driver # appended
    drivers_share_buckets: True
    # Number of requests processed in parallel by each driver
    workers: 3
    # Remove all generated buckets and its content after run
    clean_after: True

```