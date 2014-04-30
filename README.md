tensile
=======

Tensile Web Stress Test Tool

Example usage:

    tensile -help
    Usage of tensile:
      -concurrent=5: Maximum concurrent requests
      -reqs=50: Total requests
      -url="http://localhost/": Target URL
    

    
    tensile -concurrent=200 -reqs=1000

            Tensile web stress test tool v0.1
    
    Sending 1000 requests to http://localhost/ with 200 concurrent workers.
    Waiting for replies...
    
    Connections:    1000
    Concurrent:     200
    Total size:     15000 bytes
    Total time:     1.9903687s
    Average time:   1.990368ms



    tensile -concurrent=200 -reqs=100

            Tensile web stress test tool v0.1

    NOTICE: Concurrent requests is greater than number of requests.
            Changing concurrent requests to number of requests
    
    Sending 100 requests to http://localhost/ with 100 concurrent workers.
    Waiting for replies...
    
    Connections:    100
    Concurrent:     100
    Total size:     1500 bytes
    Total time:     197.1718ms
    Average time:   1.971718ms

*WARNING: This tool can rapidly deplete system resources with too many concurrent workers*