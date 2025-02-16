### Greenlight API
This is a sample API for managing a movie catalog. The purpose is only for showing how to create an API server with production level standards. 

# Reviewed Topics 
- Unit Testing
- Cuncurrency and locks
- Dependecy Injection
- MakeFile sample for golang application

- Authentication ( Basic Auth, Bearer Token, JWT)
  - Considered StatusCodes unAuthorized.
  - Don't forget to set www-authenticate header with value of Bearer, JWT or .... to show the client possible supported authentication method

- Authorization
- httprouter usage
- Implemented circuit breaking pattern for mail service
- Handling the versioning of the API
- Hanlding Json Respone and status and try to envelope responses
- Middleware usage and nested handler approaches
- Implement Json Input validation and error handling
   - Consider custom json marshaler for some types if it is necessary ( implement MarshalJSON()([]byte, error) method for them )
   - json unmarshaling errors
       - disallowUnknownFields()
       - *json.SyntaxError
       - *json.UnmarshalTypeError
       - *jsonInvalidUnmarshalError
       -  empty json body error
       -  Try to consider a json request body size to avoid DDOS and use json.Decode() method multiple times to make sure there are no multple json values in body ( just have a single json value ) exp -d '{"Title":"Moana"}', -d '{"Title":"Moana"} :asx'
       - golang validation package
       - use Location header as one of your responses like /v1/movies/id to show the client location of the created resource

- Timeouts and contexts

- Writing CORS Header: allow other domains to render a content from ur API server in the browser by adding the required headers such as below.
  - Access-Control-Allow-Origin: https://foo.com
  - Access-Control-Allow-Headers: "Content-Type, Api_Key, Authorization"
  - Access-Control-Allow-Methods: "GET, POST, PUT, PATCH, DELETE, OPTION, HEAD"
  - Consider you code to support multiple dynamic origins ( browers can't understand multiple Access-Control-Allow-Origin value separated by space or comma ) so create a list of allowed origins and check the one client has provided
  - Use "Vary: Origin" header to tell all caching servers that responses should be separately cached based on request Origin Header
  - Consider Origin for preflight requests that have specific origin headers

- Filtering , Sorting & Pagination
  - Get the query parameters based on logic
  - Get sort, page, page_size query parameters
  - Validate the query parameters provided
  - Use full-text search for partial query parameters in filters
  - For pagination use offset and limits ( offset = page-1 * pagesize ).
  - Try to validate page and pagesize to avoid integer overflow when you are multiplying them in offset
  - For the pagination try to create metadata and put it in the response to give client better view of pages. current and last page numbers, and the total number of available records
- Http Panic Recovery
  - Try to create a middleware using recover to send internal server error instead of emptyReply when Panics happens in http server you have.  - Set "Connection: close" header to make http handler automatically close the connection when panic happens in one single request go rouinte

- API RateLimiting:
  - Use rate limit package and middleware to apply global, per client rate limits
  - For per client rate limiting consider a timout to delete client ip you preserve after no access for a period of time.
  - In case you are using loadbalancer you can move to loadbalancer rateLimiting and omit the implementation from within the code.

- Graceful shutdown
  - catch signals like SIGTERM, SIGINT
  - Use http.server shutdown() method to handle this

- Database Interactions
  - Handling database connections pools ( maxIdletimeout, maxConn, MaxIdleConn &... )
  - Consider using migration tools to create your database schemas such as go-migrate , atlas, goose &....
  - Handle concurrency in case two clients want to edit same record ( deadlocks )
  - Handle long-running queries with timeouts context
  - Consider Optimistic Lock for concurrent Update request to the api ( for example query with id = ? and version = ? to always update in case version is specific to what u want and nothing has changed).
  - Consider indexing for ur database

- Client IP perservation X-Forwarded-For , X-Real-IP headers
- Implement Tracing & Metric exposure using OpenTelemetry and OpenTelemetry Collector

- Adding Prometheus Metrics
  - db.Stats()
  - http metrics
    - http total requests
    - http total requests per endpoint of API
    - http processing time second total ( Histogram or Summary metric )
    - http processing time seconds per endpoint of API
    - http response based on status code per endpoint of API ( wrap responseWriter or httpsnoop package )
    - runtime.Goroutines which shows number of go routines
    - Total number of active requests ( total requests - total responses )

- SWAGGER, SWAGGO and OPENAPI standards for API documentation

 