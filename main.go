package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type IndexPageData struct {
	Username string
	Products []Product
}

type CartPageData struct {
	CartItems []CartItem
	User      User
}

type TxWithItems struct {
	Id        int
	CreatedAt string
	Items     []LineItem
	Total     int
}

type TransactionsPageData struct {
	User    User
	History []TxWithItems
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("./templates/index.html")
	if err != nil {
		log.Fatal(err)
	}
	cookies := r.Cookies()
	var sessionToken string
	for _, cookie := range cookies {
		if cookie.Name == "cafego_session" {
			sessionToken = cookie.Value
			break
		}
	}
	user := getUserFromSessionToken(sessionToken)
	sampleProducts := getProducts()
	samplePageData := IndexPageData{Username: user.Username, Products: sampleProducts}
	err = tmpl.Execute(w, samplePageData)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	initDB()
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/product/", productHandler)
	http.HandleFunc("/login/", loginHandler)
	http.HandleFunc("/cart/", cartHandler)
	http.HandleFunc("/transactions/", transactionsHandler)
	http.ListenAndServe(":3000", nil)
}

func productHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// Get the product ID
		reqPath := r.URL.Path
		splitPath := strings.Split(reqPath, "/")
		elemCount := len(splitPath)
		// Do note that this will be a string.
		productId := splitPath[elemCount-1]
		// Need to convert from string to int
		intId, err := strconv.Atoi(productId)
		if err != nil {
			log.Fatal(err)
		}
		// Predeclare a product
		var product Product
		// Check each product for whether it matches the given ID
		for _, p := range getProducts() {
			if p.Id == intId {
				product = p
				break
			}
		}
		// If the for loop failed, then product will be the "zero-value" of the Product struct
		if product == (Product{}) {
			log.Fatal("Can't find product with that ID")
		}
		// Template rendering
		tmpl, err := template.ParseFiles("./templates/product.html")
		if err != nil {
			log.Fatal(err)
		}
		err = tmpl.Execute(w, product)
		if err != nil {
			log.Fatal(err)
		}
	} else if r.Method == "POST" {
		// Get user
		// This is copy pasted from indexHandler, so you might want to consider extracting this into its own function. I will keep this as is.
		cookies := r.Cookies()
		var sessionToken string
		for _, cookie := range cookies {
			if cookie.Name == "cafego_session" {
				sessionToken = cookie.Value
				break
			}
		}
		user := getUserFromSessionToken(sessionToken)
		userId := user.Id
		// Get product ID
		sProductId := r.FormValue("product_id")
		productId, err := strconv.Atoi(sProductId)
		if err != nil {
			log.Fatal(err)
		}
		// Get quantity
		sQuantity := r.FormValue("quantity")
		quantity, err := strconv.Atoi(sQuantity)
		if err != nil {
			log.Fatal(err)
		}
		createCartItem(userId, productId, quantity)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		tmpl, err := template.ParseFiles("./templates/login.html")
		if err != nil {
			log.Fatal(err)
		}
		err = tmpl.Execute(w, nil)
		if err != nil {
			log.Fatal(err)
		}
	} else if r.Method == "POST" {
		// In the POST arm of `loginHandler`
		rUsername := r.FormValue("username")
		rPassword := r.FormValue("password")
		var user User
		for _, u := range getUsers() {
			if (rUsername == u.Username) && (rPassword == u.Password) {
				user = u
			}
		}
		if user == (User{}) {
			fmt.Fprint(w, "Invalid login. Please go back and try again.")
			return
		}
		token := generateSessionToken()
		setSession(token, user)
		cookie := http.Cookie{Name: "cafego_session", Value: token, Path: "/"}
		http.SetCookie(w, &cookie)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func generateSessionToken() string {
	rawBytes := make([]byte, 16)
	_, err := rand.Read(rawBytes)
	if err != nil {
		log.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(rawBytes)
}

func cartHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		tmpl, err := template.ParseFiles("./templates/cart.html")
		if err != nil {
			log.Fatal(err)
		}

		cookies := r.Cookies()
		var sessionToken string
		for _, cookie := range cookies {
			if cookie.Name == "cafego_session" {
				sessionToken = cookie.Value
				break
			}
		}
		user := getUserFromSessionToken(sessionToken)

		cartItems := getCartItemsByUser(user)

		pageData := CartPageData{
			CartItems: cartItems,
			User:      user,
		}
		tmpl.Execute(w, pageData)
	} else if r.Method == "POST" {
		cookies := r.Cookies()
		var sessionToken string
		for _, cookie := range cookies {
			if cookie.Name == "cafego_session" {
				sessionToken = cookie.Value
				break
			}
		}
		user := getUserFromSessionToken(sessionToken)

		checkoutItemsForUser(user)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func transactionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// get user from session cookie
		cookies := r.Cookies()
		var sessionToken string
		for _, cookie := range cookies {
			if cookie.Name == "cafego_session" {
				sessionToken = cookie.Value
				break
			}
		}
		user := getUserFromSessionToken(sessionToken)
		if user == (User{}) {
			// not logged in — simple redirect
			http.Redirect(w, r, "/login/", http.StatusFound)
			return
		}

		// fetch transactions
		txs := getTransactionsByUser(user)
		var history []TxWithItems
		for _, t := range txs {
			items := getLineItemsByTransaction(t.Id)
			total := 0
			for _, it := range items {
				total += it.Price * it.Quantity
			}
			history = append(history, TxWithItems{
				Id:        t.Id,
				CreatedAt: t.CreatedAt,
				Items:     items,
				Total:     total,
			})
		}

		page := TransactionsPageData{
			User:    user,
			History: history,
		}

		tmpl, err := template.ParseFiles("./templates/transactions.html")
		if err != nil {
			log.Fatal(err)
		}
		if err := tmpl.Execute(w, page); err != nil {
			log.Fatal(err)
		}
	}
}
