module twitter-scraping-project

go 1.23.2

require (
	github.com/imperatrona/twitter-scraper v0.0.14
	github.com/joho/godotenv v1.5.1
)

require (
	github.com/AlexEidt/Vidio v1.5.1 // indirect
	golang.org/x/net v0.29.0 // indirect
)

// Replace the remote module with your local copy
replace github.com/imperatrona/twitter-scraper => ../twitter-scraper/library
