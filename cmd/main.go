package main

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Defines a "model" that we can use to communicate with the
// frontend or the database
// More on these "tags" like `bson:"_id,omitempty"`: https://go.dev/wiki/Well-known-struct-tags
type BookStore struct {
	MongoID     primitive.ObjectID `bson:"_id,omitempty" json:"mongo_id,omitempty"`
	ID          string             `json:"id"`
	BookName    string             `json:"title"`
	BookAuthor  string             `json:"author"`
	BookEdition string             `json:"edition"`
	BookPages   string             `json:"pages"`
	BookYear    string             `json:"year"`
}

// Wraps the "Template" struct to associate a necessary method
// to determine the rendering procedure
type Template struct {
	tmpl *template.Template
}

// Preload the available templates for the view folder.
// This builds a local "database" of all available "blocks"
// to render upon request, i.e., replace the respective
// variable or expression.
// For more on templating, visit https://jinja.palletsprojects.com/en/3.0.x/templates/
// to get to know more about templating
// You can also read Golang's documentation on their templating
// https://pkg.go.dev/text/template
func loadTemplates() *Template {
	return &Template{
		tmpl: template.Must(template.ParseGlob("views/*.html")),
	}
}

// Method definition of the required "Render" to be passed for the Rendering
// engine.
// Contraire to method declaration, such syntax defines methods for a given
// struct. "Interfaces" and "structs" can have methods associated with it.
// The difference lies that interfaces declare methods whether struct only
// implement them, i.e., only define them. Such differentiation is important
// for a compiler to ensure types provide implementations of such methods.
func (t *Template) Render(w io.Writer, name string, data interface{}, ctx echo.Context) error {
	return t.tmpl.ExecuteTemplate(w, name, data)
}

// Here we make sure the connection to the database is correct and initial
// configurations exists. Otherwise, we create the proper database and collection
// we will store the data.
// To ensure correct management of the collection, we create a return a
// reference to the collection to always be used. Make sure if you create other
// files, that you pass the proper value to ensure communication with the
// database
// More on what bson means: https://www.mongodb.com/docs/drivers/go/current/fundamentals/bson/
func prepareDatabase(client *mongo.Client, dbName string, collecName string) (*mongo.Collection, error) {
	db := client.Database(dbName)

	names, err := db.ListCollectionNames(context.TODO(), bson.D{{}})
	if err != nil {
		return nil, err
	}
	if !slices.Contains(names, collecName) {
		cmd := bson.D{{"create", collecName}}
		var result bson.M
		if err = db.RunCommand(context.TODO(), cmd).Decode(&result); err != nil {
			log.Fatal(err)
			return nil, err
		}
	}

	coll := db.Collection(collecName)
	return coll, nil
}

// Here we prepare some fictional data and we insert it into the database
// the first time we connect to it. Otherwise, we check if it already exists.
func prepareData(client *mongo.Client, coll *mongo.Collection) {
	startData := []BookStore{
		{
			ID:          "example1",
			BookName:    "The Vortex",
			BookAuthor:  "José Eustasio Rivera",
			BookEdition: "958-30-0804-4",
			BookPages:   "292",
			BookYear:    "1924",
		},
		{
			ID:          "example2",
			BookName:    "Frankenstein",
			BookAuthor:  "Mary Shelley",
			BookEdition: "978-3-649-64609-9",
			BookPages:   "280",
			BookYear:    "1818",
		},
		{
			ID:          "example3",
			BookName:    "The Black Cat",
			BookAuthor:  "Edgar Allan Poe",
			BookEdition: "978-3-99168-238-7",
			BookPages:   "280",
			BookYear:    "1843",
		},
	}

	// This syntax helps us iterate over arrays. It behaves similar to Python
	// However, range always returns a tuple: (idx, elem). You can ignore the idx
	// by using _.
	// In the topic of function returns: sadly, there is no standard on return types from function. Most functions
	// return a tuple with (res, err), but this is not granted. Some functions
	// might return a ret value that includes res and the err, others might have
	// an out parameter.
	for _, book := range startData {
		cursor, err := coll.Find(context.TODO(), book)
		var results []BookStore
		if err = cursor.All(context.TODO(), &results); err != nil {
			panic(err)
		}
		if len(results) > 1 {
			log.Fatal("more records were found")
		} else if len(results) == 0 {
			result, err := coll.InsertOne(context.TODO(), book)
			if err != nil {
				panic(err)
			} else {
				fmt.Printf("%+v\n", result)
			}

		} else {
			for _, res := range results {
				cursor.Decode(&res)
				fmt.Printf("%+v\n", res)
			}
		}
	}
}

// Generic method to perform "SELECT * FROM BOOKS" (if this was SQL, which
// it is not :D ), and then we convert it into an array of map. In Golang, you
// define a map by writing map[<key type>]<value type>{<key>:<value>}.
// interface{} is a special type in Golang, basically a wildcard...
func findAllBooks(coll *mongo.Collection) []map[string]interface{} {
	cursor, err := coll.Find(context.TODO(), bson.D{{}})
	var results []BookStore
	if err = cursor.All(context.TODO(), &results); err != nil {
		panic(err)
	}

	var ret []map[string]interface{}
	for _, res := range results {
		ret = append(ret, map[string]interface{}{
			"id":      res.ID,
			"title":   res.BookName,
			"author":  res.BookAuthor,
			"pages":   res.BookPages,
			"edition": res.BookEdition,
			"year":    res.BookYear,
		})
	}

	return ret
}

func findAllAuthors(coll *mongo.Collection) []map[string]interface{} {
	books := findAllBooks(coll)
	uniqueAuthorsMap := make(map[string]bool)

	for _, book := range books {
		if author, ok := book["author"].(string); ok {
			uniqueAuthorsMap[author] = true
		}
	}

	var ret []map[string]interface{}
	for author := range uniqueAuthorsMap {
		ret = append(ret, map[string]interface{}{"AuthorName": author})
	}

	return ret
}

func findAllYears(coll *mongo.Collection) []map[string]interface{} {
	books := findAllBooks(coll)
	uniqueYearsMap := make(map[string]bool)

	for _, book := range books {
		// Assuming "BookYear" is a field in your book map
		// and its value is a string.
		// You might need to adjust the key and type assertion
		// if your data structure is different.
		if year, ok := book["year"].(string); ok {
			uniqueYearsMap[year] = true
		}
	}

	var ret []map[string]interface{}
	for year := range uniqueYearsMap {
		ret = append(ret, map[string]interface{}{"BookYear": year})
	}

	return ret
}

func main() {
	// Connect to the database. Such defer keywords are used once the local
	// context returns; for this case, the local context is the main function
	// By user defer function, we make sure we don't leave connections
	// dangling despite the program crashing. Isn't this nice? :D
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// TODO: make sure to pass the proper username, password, and port
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))

	// This is another way to specify the call of a function. You can define inline
	// functions (or anonymous functions, similar to the behavior in Python)
	defer func() {
		if err = client.Disconnect(ctx); err != nil {
			panic(err)
		}
	}()

	// You can use such name for the database and collection, or come up with
	// one by yourself!
	coll, err := prepareDatabase(client, "exercise-1", "information")

	prepareData(client, coll)

	// Here we prepare the server
	e := echo.New()

	// Define our custom renderer
	e.Renderer = loadTemplates()

	// Log the requests. Please have a look at echo's documentation on more
	// middleware
	e.Use(middleware.Logger())

	e.Static("/css", "css")

	// Endpoint definition. Here, we divided into two groups: top-level routes
	// starting with /, which usually serve webpages. For our RESTful endpoints,
	// we prefix the route with /api to indicate more information or resources
	// are available under such route.
	e.GET("/", func(c echo.Context) error {
		return c.Render(200, "index", nil)
	})

	e.GET("/books", func(c echo.Context) error {
		books := findAllBooks(coll)
		print(books)
		return c.Render(200, "book-table", books)
	})

	e.GET("/authors", func(c echo.Context) error {
		authors := findAllAuthors(coll)
		return c.Render(200, "author-table", authors)
	})

	e.GET("/years", func(c echo.Context) error {
		years := findAllYears(coll)
		return c.Render(200, "year-table", years)
	})

	e.GET("/search", func(c echo.Context) error {
		return c.Render(200, "search-bar", nil)
	})

	e.GET("/create", func(c echo.Context) error {
		return c.NoContent(http.StatusNoContent)
	})

	// You will have to expand on the allowed methods for the path
	// `/api/route`, following the common standard.
	// A very good documentation is found here:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Methods
	// It specifies the expected returned codes for each type of request
	// method.
	e.GET("/api/books", func(c echo.Context) error {
		books := findAllBooks(coll)
		return c.JSON(http.StatusOK, books)
	})
	e.POST("/api/books", func(c echo.Context) error {
		book := new(BookStore)
		if err := c.Bind(book); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
		}

		// Generate a new ObjectID for MongoDB
		book.MongoID = primitive.NewObjectID()

		// We should also ensure the plain ID field is set, perhaps from the payload or generated.
		// For now, let's assume it might come from the payload or needs a generation strategy.
		// If ID is meant to be unique and user-provided, ensure it's present.
		// If it's to be generated, you'd add logic here.
		// For simplicity, if BookStore.ID is empty, we can use the MongoID as a string.
		if book.ID == "" {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create book"})
		}
		// Check if a book with the same ID already exists
		var existingBook BookStore
		err := coll.FindOne(context.TODO(), bson.M{"id": book.ID}).Decode(&existingBook)
		if err == nil {
			// A book with this ID already exists
			return c.JSON(http.StatusConflict, map[string]string{"error": "Book with ID " + book.ID + " already exists"})
		} else if err != mongo.ErrNoDocuments {
			// Some other error occurred during the find operation
			log.Printf("Error checking for existing book: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create book due to a database error"})
		}

		insertResult, err := coll.InsertOne(context.TODO(), book)
		if err != nil {
			log.Printf("Error inserting book: %v", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create book"})
		}

		// Optionally, you can retrieve the inserted document to return it fully populated
		// For now, we'll return the input book struct, which now includes the MongoID
		log.Printf("Inserted a single document: %v", insertResult.InsertedID)
		return c.JSON(http.StatusCreated, book)
	})

	e.PUT("/api/books/:id", func(c echo.Context) error {
		idParam := c.Param("id") // This is the custom string ID, e.g., "asd34343"

		var requestPayload map[string]interface{}
		if err := c.Bind(&requestPayload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
		}

		// The document in the database will be identified by idParam.
		filter := bson.M{"id": idParam}

		// Dynamically build the $set operation based on fields present in the request.
		updateSet := bson.M{}

		if title, ok := requestPayload["title"].(string); ok {
			updateSet["bookname"] = title // Use BSON field name "bookname"
		}
		if author, ok := requestPayload["author"].(string); ok {
			updateSet["bookauthor"] = author // Use BSON field name "bookauthor"
		}
		if edition, ok := requestPayload["edition"].(string); ok {
			updateSet["bookedition"] = edition // Use BSON field name "bookedition"
		}
		if pages, ok := requestPayload["pages"].(string); ok {
			updateSet["bookpages"] = pages // Use BSON field name "bookpages"
		}
		if year, ok := requestPayload["year"].(string); ok {
			updateSet["bookyear"] = year // Use BSON field name "bookyear"
		}

		// If no valid fields to update were provided in the request body
		if len(updateSet) == 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "No valid fields provided for update"})
		}

		update := bson.M{"$set": updateSet}

		updateResult, err := coll.UpdateOne(context.TODO(), filter, update)
		if err != nil {
			log.Printf("Error updating book with ID %s: %v", idParam, err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update book"})
		}

		if updateResult.MatchedCount == 0 {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Book not found with ID " + idParam})
		}

		// Fetch the updated document from the database to return it
		var updatedBookFromDB BookStore
		err = coll.FindOne(context.TODO(), bson.M{"id": idParam}).Decode(&updatedBookFromDB)
		if err != nil {
			log.Printf("Error fetching updated book with ID %s after update: %v", idParam, err)
			// This might indicate a race condition or an unexpected state if MatchedCount was > 0.
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to retrieve updated book details"})
		}

		return c.JSON(http.StatusOK, updatedBookFromDB)
	})
	e.DELETE("/api/books/:id", func(c echo.Context) error {
		idParam := c.Param("id") // This is the custom string ID

		filter := bson.M{"id": idParam}

		deleteResult, err := coll.DeleteOne(context.TODO(), filter)
		if err != nil {
			log.Printf("Error deleting book with ID %s: %v", idParam, err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete book"})
		}

		if deleteResult.DeletedCount == 0 {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Book not found with ID " + idParam})
		}

		return c.NoContent(http.StatusOK)
	})

	// We start the server and bind it to port 3030. For future references, this
	// is the application's port and not the external one. For this first exercise,
	// they could be the same if you use a Cloud Provider. If you use ngrok or similar,
	// they might differ.
	// In the submission website for this exercise, you will have to provide the internet-reachable
	// endpoint: http://<host>:<external-port>
	e.Logger.Fatal(e.Start(":3030"))
}
