tensile
=======

Tensile Web Stress Test Tool

Example usage:

    $ tensile -help
    Usage of tensile:
      -c=5: Maximum concurrent requests (short flag
      -concurrent=5: Maximum concurrent requests
      -cpu=4: Number of CPUs
      -r=50: Total requests (short flag)
      -request=50: Total requests
      -u="http://localhost/": Target URL (short flag)
      -url="http://localhost/": Target URL
    

    
    $ tensile -concurrent=200 -reqs=1000 -cpu=2

            Tensile web stress test tool v0.1
    
    Sending 1000 requests to http://localhost/ with 200 concurrent workers using 2 CPUs.
    Waiting for replies...
    
    Connections:    1000
    Concurrent:     200
    Total size:     15000 bytes
    Total time:     1.9903687s
    Average time:   1.990368ms



    $ tensile -c=200 -r=100

            Tensile web stress test tool v0.1

    NOTICE: -concurrent=200 is greater than -reqs
            Changing -concurrent to 100
    
    Sending 100 requests to http://localhost/ with 100 concurrent workers using 4 CPUs.
    Waiting for replies...
    
    Connections:    100
    Concurrent:     100
    Total size:     1500 bytes
    Total time:     197.1718ms
    Average time:   1.971718ms

*WARNING: This tool can rapidly deplete system resources with too many concurrent workers*