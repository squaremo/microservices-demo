package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

var customerSearchPath = "/customers/search/findByUsername"
var customersPath = "/customers"
var addressesPath = "/addresses"
var cardsPath = "/cards"

var customerHost = "accounts"
var dev bool
var verbose bool
var port string
var users []user

func main() {

	flag.StringVar(&port, "port", "8084", "Port on which to run")
	flag.BoolVar(&dev, "dev", false, "Run in development mode")
	flag.BoolVar(&verbose, "verbose", false, "Verbose logging")
	flag.Parse()

	var file string
	if dev {
		file = "./users.json"
	} else {
		file = "/config/users.json"
	}
	loadUsers(file)

	if dev {
		customerHost = "192.168.99.102:32769"
	}

	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/register", registerHandler)
	log.Printf("Login service running on port %s\n", port)
	http.ListenAndServe(":"+port, nil)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Request received")
	u, p, ok := r.BasicAuth()
	if !ok {
		log.Printf("No Authorization header present.\n")
		w.WriteHeader(401)
		return
	}

	if dev || verbose {
		log.Printf("Lookup for user %s and password: %s.\n", u, p)
	}
	if !validatePassword(u, p) {
		log.Printf("User not authorized.\n")
		w.WriteHeader(401)
		return
	}

	c, err := lookupCustomer(u, p)

	if err != nil {
		w.WriteHeader(401)
		panic(err)
	}

	if len(c.Embedded.Customers) < 1 {
		log.Printf("No customer found for that username")
		w.WriteHeader(401)
		return
	}

	cust := c.Embedded.Customers[0]
	custLink := cust.Links.CustomerLink.Href

	idSplit := strings.Split(custLink, "/")
	id := idSplit[len(idSplit)-1]
	if dev || verbose {
		log.Printf("Customer id: %s\n", id)
	}
	var res response
	res.Username = cust.Username
	res.Customer = custLink
	res.Id = id

	js, err := json.Marshal(res)

	if err != nil {
		w.WriteHeader(401)
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)

	var b registerBody
	err := decoder.Decode(&b)

	if err != nil {
		log.Printf("No request body found.\n" + err.Error())
		w.WriteHeader(400)
		return
	}

	addressLink, err := createAddress(b.Address)
	if err != nil {
		log.Printf(err.Error())
		w.WriteHeader(400)
		return
	}
	cardLink, err := createCard(b.Card)
	if err != nil {
		log.Printf(err.Error())
		w.WriteHeader(400)
		return
	}
	c := b.Customer
	username := c.Username
	password := c.Password
	c.Password = ""
	c.Addresses = []string{addressLink}
	c.Cards = []string{cardLink}

	if !createCustomer(c) {
		log.Println("Customer not created")
		w.WriteHeader(400)
		return
	}

	users = append(users, user{Id: "", Name: username, Password: password})

	w.WriteHeader(200)
}

func createAddress(a address) (string, error) {
	jsonBytes, err := json.Marshal(a)
	if err != nil {
		panic(err)
	}
	url := "http://" + customerHost + addressesPath

	if dev || verbose {
		fmt.Printf("POSTing %v\n", string(jsonBytes))
		fmt.Println("URL: " + url)
	}

	res, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", err
	}

	return res.Header.Get("Location"), nil
}

func createCard(c card) (string, error) {
	jsonBytes, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}

	res, err := http.Post("http://" + customerHost + addressesPath, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return "", err
	}

	return res.Header.Get("Location"), nil
}

func createCustomer(c customer) bool {
	jsonBytes, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}

	if dev || verbose {
		fmt.Println("Posting Customer: " + string(jsonBytes))
	}
	res, err := http.Post("http://" + customerHost + addressesPath, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		panic(err)
	}
	if res.StatusCode == 200 || res.StatusCode == 201 {
		return true
	}
	return false
}


func lookupCustomer(u, p string) (customerResponse, error) {
	var c customerResponse

	reqUrl := "http://" + customerHost + customerSearchPath + "?username=" + u
	log.Println(reqUrl)
	res, err := http.Get(reqUrl)
	if err != nil {
		return c, err
	}

	defer res.Body.Close()
	json.NewDecoder(res.Body).Decode(&c)

	if dev || verbose {
		log.Printf("Received response: %v\n", c)
	}

	if err != nil {
		return c, err
	}

	return c, nil
}

func validatePassword(u, p string) bool{
	for _, user := range users {
		if user.Name == u && user.Password == p {
			return true
		}
	}
	return false
}

func loadUsers(file string) {
	f, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}
	json.Unmarshal(f, &users)
	log.Printf("Loaded %d users.", len(users))
}

type customerResponse struct {
	Embedded struct {
		Customers []struct {
			Username string `json:"username"`
			Links    struct {
				CustomerLink struct {
					Href string `json:"href"`
				} `json:"customer"`
			} `json:"_links"`
			} `json:"customer"`
	} `json:"_embedded"`
}

type user struct {
	Id       string `json:"id"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

type registerBody struct {
	Address address `json:"address"`
	Card card `json:"card"`
	Customer customer `json:"customer"`
}

type customer struct {
	FirstNmae string `json:"firstName"`
	LastName string `json:"lastName"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	Addresses []string `json:"addresses"`
	Cards []string `json:"cards"`
}

type address struct {
	Street string `json:"street"`
	Number string `json:"number"`
	Country string `json:"country"`
	City string `json:"city"`
}

type card struct {
	LongNum string `json:"longNum"`
	Expires string `json:"expires"`
	Ccv string `json:"ccv"`
}

type response struct {
	Username string `json:"username"`
	Customer string `json:"customer"`
	Id       string `json:"id"`
}
