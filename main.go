package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/gocolly/colly"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Stock struct {
	Company, Price, Change string
}

var stocks []Stock
var mu sync.Mutex

func main() {
	app := fiber.New()

	ticker := []string{
		"MSFT",
		"IBM",
		"GE",
		"UNP",
		"COST",
		"MCD",
		"V",
		"WMT",
		"DIS",
		"MMM",
		"INTC",
		"AXP",
		"AAPL",
		"BA",
		"CSCO",
		"GS",
		"JPM",
		"CRM",
		"VZ",
	}

	c := colly.NewCollector()

	c.OnRequest(func(r *colly.Request) {
		fmt.Println("Visiting:", r.URL)
	})

	c.OnError(func(_ *colly.Response, err error) {
		log.Println("Something went wrong: ", err)
	})

	c.OnHTML("div#quote-header-info", func(e *colly.HTMLElement) {
		stock := Stock{}
		stock.Company = e.ChildText("h1")
		stock.Price = e.ChildText("fin-streamer[data-field='regularMarketPrice']")
		stock.Change = e.ChildText("fin-streamer[data-field='regularMarketChangePercent']")

		mu.Lock()
		stocks = append(stocks, stock)
		mu.Unlock()
	})

	for _, t := range ticker {
		c.Visit("https://finance.yahoo.com/quote/" + t + "/")
	}

	app.Get("/stocks", func(c *fiber.Ctx) error {
		return c.JSON(stocks)
	})

	// MongoDB connection options
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		return
	}
	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Disconnect(context.Background())

	// Access the "stocks" collection in the "mydb" database
	collection := client.Database("stockdb").Collection("stocks")

	// Insert the scraped stock data into MongoDB
	var bsonStocks []interface{}
	for _, s := range stocks {
		bsonStock, _ := bson.Marshal(s)
		var unmarshaledStock interface{}
		_ = bson.Unmarshal(bsonStock, &unmarshaledStock)
		bsonStocks = append(bsonStocks, unmarshaledStock)
	}

	_, err = collection.InsertMany(context.Background(), bsonStocks)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Server is running...")
	err = app.Listen(":3000")
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}
