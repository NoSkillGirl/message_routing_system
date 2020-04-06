package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	// "firebase.google.com/go/db"
	_ "github.com/mattn/go-sqlite3"
)

func init() {
	initSQLite()
	initializeRoutes()
}

func initSQLite() {
	database, _ := sql.Open("sqlite3", "./question.db")
	createGateWayQuery, _ := database.Prepare("CREATE TABLE IF NOT EXISTS gateways (id INTEGER PRIMARY KEY AUTOINCREMENT, name VARCHAR(255))")
	createGateWayQuery.Exec()
	createGateWayIPAdressesQuery, _ := database.Prepare("CREATE TABLE IF NOT EXISTS gateway_ip_addresses (id INTEGER PRIMARY KEY AUTOINCREMENT, gateway_id INTEGER, ip VARCHAR(16))")
	createGateWayIPAdressesQuery.Exec()
	createRouteMappingQuery1, _ := database.Prepare("CREATE TABLE IF NOT EXISTS route_mapping (id INTEGER PRIMARY KEY AUTOINCREMENT, gateway_id INTEGER, prefix VARCHAR(16))")
	createRouteMappingQuery1.Exec()
}

func initializeRoutes() {
	http.HandleFunc("/gateway/", createGateWay)
	http.HandleFunc("/route/", createRouteMapping)
	http.HandleFunc("/search/route/", searchRoute)
}

func getDatabaseConnection() (*sql.DB, error) {
	database, err := sql.Open("sqlite3", "./question.db")
	if err != nil {
		return nil, err
	}
	return database, nil
}

func main() {
	http.ListenAndServe(":8000", nil)

}

type createGateWayRequest struct {
	Name        string   `json:"name,required"`
	IpAddresses []string `json:"ip_addresses,required"`
}

type createGateWayResponse struct {
	Message string
	Param   string
}

type createGateWaySuccessResponse struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	IPAddresses []string `json:"ip_addresses"`
}

type showGateWayRequest struct {
	ID int
}

type createRouteMappingRequest struct {
	Prefix    string `json:"prefix"`
	GateWayId int    `json:"gateway_id"`
}

type createRouteMappingResponse struct {
	ID      int
	Prefix  string
	Gateway createGateWaySuccessResponse
}

func searchRoute(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		num := strings.Split(r.URL.Path, "/")[3]
		// num := "918008270250"

		validationErrorRes := createGateWayResponse{}
		successRes := createRouteMappingResponse{}

		nonDigitChar, _ := regexp.MatchString(`[^\d]`, num)
		if len(num) > 12 || nonDigitChar {
			validationErrorRes.Message = "Invalid Phone number"
			w.WriteHeader(http.StatusBadRequest)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(validationErrorRes)
			return
		}

		db, databaseConnErr := getDatabaseConnection()
		// If there is a issue getting the database connection
		if databaseConnErr != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Get all the prefixes from the table
		getAllPrefixesQueryString := "SELECT id, prefix FROM route_mapping"
		rows, err := db.Query(getAllPrefixesQueryString)
		if err != nil {
			if err == sql.ErrNoRows {
				validationErrorRes.Message = "no routes exist in the database"
				w.WriteHeader(http.StatusBadRequest)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(validationErrorRes)
				return
			}
			fmt.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		idPrefixMap := make(map[string]string)
		for rows.Next() {
			var id, prefix string
			rows.Scan(&id, &prefix)
			idPrefixMap[id] = prefix
		}

		var routeIDInt int

		for k, v := range idPrefixMap {
			if strings.Contains(num, v) {
				kInt, _ := strconv.Atoi(k)
				if kInt > routeIDInt {
					routeIDInt = kInt
				}

			}
		}

		// Get the prefix, gateway_id from the table
		getRouteNameQuery := `SELECT prefix, gateway_id FROM route_mapping where id = '%d' limit 1`
		getRouteNameQueryString := fmt.Sprintf(getRouteNameQuery, routeIDInt)
		row := db.QueryRow(getRouteNameQueryString)

		err = row.Scan(&successRes.Prefix, &successRes.Gateway.ID)
		if err != nil {
			if err == sql.ErrNoRows {
				validationErrorRes.Message = "No gateway supports this mobile number"
				w.WriteHeader(http.StatusNotFound)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(validationErrorRes)
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Get the gateway name from the table
		getGateWayNameQuery := `SELECT name FROM gateways where id = '%d' limit 1`
		getGateWayNameQueryString := fmt.Sprintf(getGateWayNameQuery, successRes.Gateway.ID)
		row = db.QueryRow(getGateWayNameQueryString)

		err = row.Scan(&successRes.Gateway.Name)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Get the gateway ip address details
		getGateWayIPQuery := `SELECT ip FROM gateway_ip_addresses where gateway_id = '%d'`
		getGateWayIPQueryString := fmt.Sprintf(getGateWayIPQuery, successRes.Gateway.ID)
		rows, err = db.Query(getGateWayIPQueryString)
		if err != nil {
			fmt.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var ipAddresses []string
		for rows.Next() {
			var ipAddress string
			rows.Scan(&ipAddress)
			ipAddresses = append(ipAddresses, ipAddress)
		}

		successRes.Gateway.IPAddresses = ipAddresses

		w.WriteHeader(http.StatusCreated)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(successRes)
	default:
		fmt.Fprintf(w, "Sorry, only GET method is supported.")
	}
}

func createGateWay(w http.ResponseWriter, r *http.Request) {
	matched, _ := regexp.MatchString(`gateway/[0-9]+`, r.URL.Path)
	if matched && r.Method == "POST" {
		fmt.Fprintf(w, "Sorry, only GET method is supported for this API.")
	}
	if !matched && r.Method == "GET" {
		fmt.Fprintf(w, "Sorry, only POST method is supported for this API.")
	}

	switch r.Method {
	case "POST":
		if !matched {
			var req createGateWayRequest
			res := createGateWayResponse{}

			decodeErr := json.NewDecoder(r.Body).Decode(&req)
			db, databaseConnErr := getDatabaseConnection()

			// If there is a issue in decoding the req or getting the database connection
			if decodeErr != nil || databaseConnErr != nil {
				res.Message = "Internal Server Error"
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// 1. Check if any gateway exists with the same name
			// 2. Name Can't be Empty
			// 3. Atleast one IP Address should be present

			validationErr := false
			validationErrReason := ""
			badIPAddress := false

			for _, address := range req.IpAddresses {
				if is_ipv4(address) {
					badIPAddress = true
					break
				}
			}

			if req.Name == "" {
				validationErr = true
				validationErrReason = "name"
			} else if badIPAddress || len(req.IpAddresses) < 1 {
				validationErr = true
				validationErrReason = "ip_addresses"
			} else {
				dbQuery := `SELECT name FROM gateways where name = '%s' limit 1`
				dbQueryString := fmt.Sprintf(dbQuery, req.Name)
				row := db.QueryRow(dbQueryString)
				var thisGateWayObj gateWay
				err := row.Scan(&thisGateWayObj.Name)

				if err != nil {
					if err == sql.ErrNoRows {
						// Insert into gateway table
						insertGateWayQuery := `INSERT INTO gateways (name) values ('%s')`
						insertGateWayQueryString := fmt.Sprintf(insertGateWayQuery, req.Name)
						insertGateWayQueryStmt, _ := db.Prepare(insertGateWayQueryString)
						insertGateWayQueryStmt.Exec()

						// Get the gateway id from the table
						getGateWayIDQuery := `SELECT id FROM gateways where name = '%s' limit 1`
						getGateWayIDQueryString := fmt.Sprintf(getGateWayIDQuery, req.Name)
						row := db.QueryRow(getGateWayIDQueryString)
						var thisGateWayObj1 gateWay
						err := row.Scan(&thisGateWayObj1.ID)
						if err != nil {
							res.Message = "Internal Server Error"
							w.WriteHeader(http.StatusInternalServerError)
							json.NewEncoder(w).Encode(res)
							return
						}

						// Insert into gateway_ip_address table
						for _, v := range req.IpAddresses {
							insertGateWayIPAddressQuery := `INSERT INTO gateway_ip_addresses (gateway_id, ip) values ('%d','%s')`
							insertGateWayIPAddressQueryString := fmt.Sprintf(insertGateWayIPAddressQuery, thisGateWayObj1.ID, v)
							insertGateWayIPAddressQueryStmt, _ := db.Prepare(insertGateWayIPAddressQueryString)
							insertGateWayIPAddressQueryStmt.Exec()
						}
						w.WriteHeader(http.StatusCreated)
						w.Header().Set("Content-Type", "application/json")
						json.NewEncoder(w).Encode(createGateWaySuccessResponse{
							ID:          thisGateWayObj1.ID,
							Name:        req.Name,
							IPAddresses: req.IpAddresses,
						})

						return

					} else {
						// If Actual Error exists
						res.Message = "Internal Server Error"
						w.WriteHeader(http.StatusInternalServerError)
						json.NewEncoder(w).Encode(res)
						return
					}
				} else {
					// Returned a row
					validationErr = true
					validationErrReason = "name"
				}
			}

			if validationErr {
				res.Message = "gateway with this name already exists"
				res.Param = validationErrReason
				w.WriteHeader(http.StatusBadRequest)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(res)
				return
			}
		}
	case "GET":
		if matched {
			gateWayID := strings.Split(r.URL.Path, "/")[2]
			gateWayIDInt, _ := strconv.Atoi(gateWayID)

			validationErrorRes := createGateWayResponse{}
			successRes := createGateWaySuccessResponse{}
			successRes.ID = gateWayIDInt

			db, databaseConnErr := getDatabaseConnection()
			// If there is a issue getting the database connection
			if databaseConnErr != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// Get the gateway id from the table
			getGateWayNameQuery := `SELECT name FROM gateways where id = '%d' limit 1`
			getGateWayNameQueryString := fmt.Sprintf(getGateWayNameQuery, gateWayIDInt)
			row := db.QueryRow(getGateWayNameQueryString)

			err := row.Scan(&successRes.Name)
			if err != nil {
				if err == sql.ErrNoRows {
					validationErrorRes.Message = "gateway with this id doesn't exist"
					validationErrorRes.Param = "gateway_id"
					w.WriteHeader(http.StatusNotFound)
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(validationErrorRes)
					return
				} else {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}

			// Get the gateway ip address details
			getGateWayIPQuery := `SELECT ip FROM gateway_ip_addresses where gateway_id = '%d'`
			getGateWayIPQueryString := fmt.Sprintf(getGateWayIPQuery, gateWayIDInt)
			rows, err := db.Query(getGateWayIPQueryString)
			if err != nil {
				fmt.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			var ipAddresses []string
			for rows.Next() {
				var ipAddress string
				rows.Scan(&ipAddress)
				ipAddresses = append(ipAddresses, ipAddress)
			}

			successRes.IPAddresses = ipAddresses

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(successRes)

		}
	default:
		fmt.Fprintf(w, "Sorry, only POST method is supported.")
	}
}

func createRouteMapping(w http.ResponseWriter, r *http.Request) {
	matched, _ := regexp.MatchString(`route/[0-9]+`, r.URL.Path)
	if matched && r.Method == "POST" {
		fmt.Fprintf(w, "Sorry, only GET method is supported for this API.")
	}
	if !matched && r.Method == "GET" {
		fmt.Fprintf(w, "Sorry, only POST method is supported for this API.")
	}

	switch r.Method {
	case "POST":
		if !matched {
			var req createRouteMappingRequest
			res := createRouteMappingResponse{}
			validationFailureRes := createGateWayResponse{}

			decodeErr := json.NewDecoder(r.Body).Decode(&req)
			db, databaseConnErr := getDatabaseConnection()

			// If there is a issue in decoding the req or getting the database connection
			if decodeErr != nil || databaseConnErr != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(res)
				return
			}

			if req.Prefix == "" {
				validationFailureRes.Message = "prefix empty"
				validationFailureRes.Param = "prefix"
				w.WriteHeader(http.StatusBadRequest)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(validationFailureRes)
				return
			} else if req.GateWayId == 0 {
				validationFailureRes.Message = "gateway empty"
				validationFailureRes.Param = "gateway"
				w.WriteHeader(http.StatusBadRequest)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(validationFailureRes)
				return
			}

			dbQuery := `SELECT id FROM route_mapping where prefix = '%s' limit 1`
			dbQueryString := fmt.Sprintf(dbQuery, req.Prefix)
			row := db.QueryRow(dbQueryString)
			var thisRouteMappingObj routeMapping
			err := row.Scan(&thisRouteMappingObj.ID)

			if err != nil {
				if err == sql.ErrNoRows {

					createRouteMappingQuery := `INSERT INTO route_mapping (prefix, gateway_id) values ('%s', '%d')`
					createRouteMappingQueryString := fmt.Sprintf(createRouteMappingQuery, req.Prefix, req.GateWayId)
					createRouteMappingQueryStmt, _ := db.Prepare(createRouteMappingQueryString)
					createRouteMappingQueryStmt.Exec()

					// Get the route details from the table
					getRouteMappingQuery := `SELECT id, prefix FROM route_mapping where prefix = '%s' limit 1`
					getRouteMappingQueryString := fmt.Sprintf(getRouteMappingQuery, req.Prefix)
					row := db.QueryRow(getRouteMappingQueryString)

					var thisRouteMappingResponse createRouteMappingResponse

					err := row.Scan(&thisRouteMappingResponse.ID, &thisRouteMappingResponse.Prefix)
					if err != nil {
						fmt.Println(err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					// Get the gateway id, name from the table
					getGateWayNameQuery := `SELECT id, name FROM gateways where id = '%d' limit 1`
					getGateWayNameQueryString := fmt.Sprintf(getGateWayNameQuery, req.GateWayId)
					row = db.QueryRow(getGateWayNameQueryString)
					err = row.Scan(&thisRouteMappingResponse.Gateway.ID, &thisRouteMappingResponse.Gateway.Name)
					if err != nil {
						fmt.Println(err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					// Get the gateway ip address details
					getGateWayIPQuery := `SELECT ip FROM gateway_ip_addresses where gateway_id = '%d'`
					getGateWayIPQueryString := fmt.Sprintf(getGateWayIPQuery, req.GateWayId)
					rows, err := db.Query(getGateWayIPQueryString)
					if err != nil {
						fmt.Println(err)
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					var ipAddresses []string
					for rows.Next() {
						var ipAddress string
						rows.Scan(&ipAddress)
						ipAddresses = append(ipAddresses, ipAddress)
					}

					thisRouteMappingResponse.Gateway.IPAddresses = ipAddresses

					w.WriteHeader(http.StatusCreated)
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(thisRouteMappingResponse)
					return

				} else {
					// If Actual Error exists
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(res)
					return
				}
			} else {
				validationFailureRes.Message = "prefix already exists"
				validationFailureRes.Param = "prefix"
				w.WriteHeader(http.StatusBadRequest)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(validationFailureRes)
				return
			}
		}
	case "GET":
		if matched {
			routeID := strings.Split(r.URL.Path, "/")[2]
			routeIDInt, _ := strconv.Atoi(routeID)

			validationErrorRes := createGateWayResponse{}
			successRes := createRouteMappingResponse{}
			successRes.ID = routeIDInt

			db, databaseConnErr := getDatabaseConnection()

			// If there is a issue getting the database connection
			if databaseConnErr != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// Get the prefix, gateway_id from the table
			getRouteNameQuery := `SELECT prefix, gateway_id FROM route_mapping where id = '%d' limit 1`
			getRouteNameQueryString := fmt.Sprintf(getRouteNameQuery, routeIDInt)
			row := db.QueryRow(getRouteNameQueryString)

			err := row.Scan(&successRes.Prefix, &successRes.Gateway.ID)
			if err != nil {
				if err == sql.ErrNoRows {
					validationErrorRes.Message = "route with this id doesn't exist"
					validationErrorRes.Param = "route_id"
					w.WriteHeader(http.StatusNotFound)
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(validationErrorRes)
				} else {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
			}

			// Get the gateway name from the table
			getGateWayNameQuery := `SELECT name FROM gateways where id = '%d' limit 1`
			getGateWayNameQueryString := fmt.Sprintf(getGateWayNameQuery, successRes.Gateway.ID)
			row = db.QueryRow(getGateWayNameQueryString)

			err = row.Scan(&successRes.Gateway.Name)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// Get the gateway ip address details
			getGateWayIPQuery := `SELECT ip FROM gateway_ip_addresses where gateway_id = '%d'`
			getGateWayIPQueryString := fmt.Sprintf(getGateWayIPQuery, successRes.Gateway.ID)
			rows, err := db.Query(getGateWayIPQueryString)
			if err != nil {
				fmt.Println(err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			var ipAddresses []string
			for rows.Next() {
				var ipAddress string
				rows.Scan(&ipAddress)
				ipAddresses = append(ipAddresses, ipAddress)
			}

			successRes.Gateway.IPAddresses = ipAddresses

			w.WriteHeader(http.StatusCreated)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(successRes)
		}
	default:
		fmt.Fprintf(w, "Sorry, only POST method is supported.")
	}

}

func is_ipv4(host string) bool {
	parts := strings.Split(host, ".")

	if len(parts) < 4 {
		return false
	}

	for _, x := range parts {
		if i, err := strconv.Atoi(x); err == nil {
			if i < 0 || i > 255 {
				return false
			}
		} else {
			return false
		}

	}
	return true
}

// Models
type gateWay struct {
	ID   int
	Name string
}

type gateWayIPAddress struct {
	ID        int
	GatewayID int
	IPAddress string
}

type routeMapping struct {
	ID        int
	gatewayID int
	Prefix    string
}
