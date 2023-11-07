package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/gocolly/colly"
	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func loadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

type Stock struct {
	Company, Price, Change string
	ScrapingDate           time.Time // Add the date when the data was scraped
}

var (
	stocks     []Stock
	stocksLock sync.Mutex
)

func main() {
	loadEnv()

	// Create a ticker that runs every 24 hours
	ticker := time.NewTicker(24 * time.Hour)

	go func() {
		for {
			scrapeAndInsertData()
			<-ticker.C // Wait for the ticker to trigger
		}
	}()

	// Start the Fiber server
	app := setupFiber()
	err := app.Listen(":8080")

	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

func scrapeAndInsertData() {
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
		stock.ScrapingDate = time.Now() // Record the scraping date

		stocksLock.Lock()
		stocks = append(stocks, stock)
		stocksLock.Unlock()
	})

	for _, t := range ticker {
		c.Visit("https://finance.yahoo.com/quote/" + t + "/")
	}

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
	stocksLock.Lock()
	for _, s := range stocks {
		bsonStock, _ := bson.Marshal(s)
		var unmarshaledStock interface{}
		_ = bson.Unmarshal(bsonStock, &unmarshaledStock)
		bsonStocks = append(bsonStocks, unmarshaledStock)
	}
	stocks = []Stock{} // Clear the stocks slice
	stocksLock.Unlock()

	_, err = collection.InsertMany(context.Background(), bsonStocks)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Scraping and data insertion completed.")

	// Start the Fiber server after scraping and data insertion
	app := setupFiber()
	err = app.Listen(":8080")
	if err != nil {
		fmt.Println("Error starting server:", err)
	}
}

func setupFiber() *fiber.App {
	app := fiber.New()

	app.Get("/stocks", func(c *fiber.Ctx) error {
		stocksLock.Lock()
		defer stocksLock.Unlock()
		return c.JSON(stocks)
	})

	fmt.Println("Fiber server is ready to serve.")
	return app
}
