package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/cybrarymin/greenlight/internal/data"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
)

type envelope map[string]interface{}

// Background function will send a function to the background using
func (app *application) BackgroundJob(nfunc func(), PanicErrMsg string) {
	app.wg.Add(1)
	go func() {
		defer app.wg.Done()
		defer func() {
			if panicErr := recover(); panicErr != nil {
				pErr := errors.New(fmt.Sprintln(panicErr))
				app.log.Error().Stack().Err(pErr).Msg(PanicErrMsg)
			}
		}()
		nfunc()
	}()
}

func (app *application) readIDParam(r *http.Request) (int64, error) {
	params := httprouter.ParamsFromContext(r.Context())
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, err
	}
	return id, nil
}

func (app *application) readUUIDParam(r *http.Request) (uuid.UUID, error) {
	params := httprouter.ParamsFromContext(r.Context())
	uuidParam := params.ByName("id")
	cuuid, err := uuid.Parse(uuidParam)
	if err != nil {
		return uuid.Nil, err
	}
	return cuuid, nil
}

// readString function reads the query strings then extracts the the value of the specified key.
// If the key doesn't exist it will return default value
func (app *application) readString(qs url.Values, key string, defaultValue string) string {
	if value := qs.Get(key); value != "" {
		return value
	}
	return defaultValue
}

// The readCSV() helper reads a string value from the query string and then splits it
// into a slice on the comma character. If no matching key could be found, it returns
// the provided default value.
func (app *application) readCSV(qs url.Values, key string, defaultValue []string) []string {
	csv := qs.Get(key)
	if csv == "" {
		return defaultValue
	}
	return strings.Split(csv, ",")
}

// The readInt() helper reads a string value from the query string and converts it to an
// integer before returning. If no matching key could be found it returns the provided
// default value. If the value couldn't be converted to an integer, then we record an
// error message in the provided Validator instance.
func (app *application) readInt(qs url.Values, key string, defaultValue int, v *data.Validator) int {
	numString := qs.Get(key)
	if numString == "" {
		return defaultValue
	}
	num, err := strconv.Atoi(numString)
	if err != nil {
		v.AddError(key, "must be an integer type")
		return defaultValue
	}
	return num
}

func (app *application) writeJson(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	nBuffer := bytes.Buffer{}
	err := json.NewEncoder(&nBuffer).Encode(data)
	if err != nil {
		return err
	}
	for key, value := range headers {
		w.Header()[key] = value
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(nBuffer.Bytes())

	return nil
}

func (app *application) readJson(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	// Limit the amount of bytes accepted as post request body
	maxBytes := 1_048_576 // _ here is only for visual separator purpose and for int values go's compiler will ignore it.
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))
	dec := json.NewDecoder(r.Body)
	// Initialize the json.Decoder, and call the DisallowUnknownFields() method on it
	// before decoding. This means that if the JSON from the client now includes any
	// field which cannot be mapped to the target destination, the decoder will return
	// an error instead of just ignoring the field.
	dec.DisallowUnknownFields()
	err := dec.Decode(&dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		switch {
		// This happens if we json syntax errors. having wrong commas or indentation or missing quotes
		case errors.As(err, &syntaxError):
			return fmt.Errorf("body contains badly-formed json (at character %d)", syntaxError.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("body contains badly-formed JSON")

		// This will happen if we try to unmarshal a json value of a type to a struct field that doesn't support that specific type
		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("invalid type used for the key %s", unmarshalTypeError.Field)
			}
			// if client provide completely different type of json. for example instead of json of object type it sends an array content json
			return fmt.Errorf("body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)

		// If the JSON contains a field which cannot be mapped to the target destination
		// then Decode() will now return an error message in the format "json: unknown
		// field "<name>"". We check for this, extract the field name from the error,
		// and interpolate it into our custom error message.
		case strings.HasPrefix(err.Error(), "json: unknown field"):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field")
			return fmt.Errorf("body contains unknown field %s", fieldName)

		// If the request body exceeds 1MB in size the decode will now fail with the
		// error "http: request body too large". There is an open issue about turning
		// this into a distinct error type at https://github.com/golang/go/issues/30715.
		case err.Error() == "http: request body too large":
			return fmt.Errorf("body must not be larger than %d bytes", maxBytes)

		// Error will happen if we pass invalid type to json.Decode function. we should always pass a pointer otherwise it will give us error
		case errors.As(err, &invalidUnmarshalError):
			panic(err)
		case errors.Is(err, io.EOF):
			return errors.New("json body must not be empty")
		default:
			return err
		}
	}

	// by default decode method of json package will read json values one by one.
	//  If the request body only contained a single JSON value this will
	// return an io.EOF error. So if we get anything else, we know that there is
	// additional data in the request body and we return our own custom error message.
	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return errors.New("body must only contain a single json value")
	}
	return nil
}

func createKeyValuePairs(m map[string]string) string {
	b := new(bytes.Buffer)
	for key, value := range m {
		fmt.Fprintf(b, "%s=\"%s\"\n", key, value)
	}
	return b.String()
}
