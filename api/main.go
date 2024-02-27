package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const DefaultPort = 8080

type UserResponse struct {
	Status  int        `json:"status"`
	Message string     `json:"message"`
	Data    *fiber.Map `json:"data"`
}

type EventLog struct {
	Id           primitive.ObjectID `json:"id,omitempty"`
	Created      string             `json:"created,omitempty" validate:"required"`
	EventName    string             `json:"event_name,omitempty" validate:"required"`
	EventDetails interface{}        `json:"event_details,omitempty" validate:"required"`
}

func envMongoURI() string {
	return os.Getenv("MONGO_URI")
}

func connectDB() *mongo.Client {
	client, err := mongo.NewClient(options.Client().ApplyURI(envMongoURI()))
	if err != nil {
		log.Fatal(err)
	}

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}

	//ping the database
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to MongoDB")
	return client
}

var DB *mongo.Client = connectDB()
var eventLogCollection *mongo.Collection = getCollection(DB, "app", "behaviour_analysis")
var validate = validator.New()

func getCollection(client *mongo.Client, dbName string, collectionName string) *mongo.Collection {
	rb := bson.NewRegistryBuilder()
	rb.RegisterTypeMapEntry(bsontype.EmbeddedDocument, reflect.TypeOf(bson.M{}))
	reg := rb.Build()
	dbOptions := options.DatabaseOptions{Registry: reg}
	collection := client.Database(dbName, &dbOptions).Collection(collectionName)
	return collection
}

func getData(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	eventLogId := c.Params("eventLogId")
	var eventLog EventLog
	defer cancel()

	objId, _ := primitive.ObjectIDFromHex(eventLogId)

	err := eventLogCollection.FindOne(ctx, bson.M{"id": objId}).Decode(&eventLog)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(UserResponse{Status: http.StatusInternalServerError, Message: "error", Data: &fiber.Map{"data": err.Error()}})
	}

	return c.Status(http.StatusOK).JSON(UserResponse{Status: http.StatusOK, Message: "success", Data: &fiber.Map{"data": eventLog}})
}

func getAllData(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var eventLogs []EventLog
	defer cancel()

	results, err := eventLogCollection.Find(ctx, bson.M{})

	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(UserResponse{Status: http.StatusInternalServerError, Message: "error", Data: &fiber.Map{"data": err.Error()}})
	}

	defer results.Close(ctx)
	for results.Next(ctx) {
		var details map[string]string
		singleEventLog := EventLog{EventDetails: &details}
		if err = results.Decode(&singleEventLog); err != nil {
			return c.Status(http.StatusInternalServerError).JSON(UserResponse{Status: http.StatusInternalServerError, Message: "error parsing top level data", Data: &fiber.Map{"data": err.Error()}})
		}
		eventLogs = append(eventLogs, singleEventLog)
	}

	return c.Status(http.StatusOK).JSON(
		UserResponse{Status: http.StatusOK, Message: "success", Data: &fiber.Map{"data": eventLogs}},
	)
}

func postData(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var eventLog EventLog
	defer cancel()

	if err := c.BodyParser(&eventLog); err != nil {
		return c.Status(http.StatusBadRequest).JSON(UserResponse{Status: http.StatusBadRequest, Message: "error", Data: &fiber.Map{"data": err.Error()}})
	}

	if validationErr := validate.Struct(&eventLog); validationErr != nil {
		return c.Status(http.StatusBadRequest).JSON(UserResponse{Status: http.StatusBadRequest, Message: "error", Data: &fiber.Map{"data": validationErr.Error()}})
	}

	newEventLog := EventLog{
		Id:           primitive.NewObjectID(),
		Created:      eventLog.Created,
		EventName:    eventLog.EventName,
		EventDetails: eventLog.EventDetails,
	}

	result, err := eventLogCollection.InsertOne(ctx, newEventLog)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(UserResponse{Status: http.StatusInternalServerError, Message: "error", Data: &fiber.Map{"data": err.Error()}})
	}

	return c.Status(http.StatusCreated).JSON(UserResponse{Status: http.StatusCreated, Message: "success", Data: &fiber.Map{"data": result}})

}

func main() {
	app := fiber.New()
	app.Get("/data", getData)
	app.Get("/alldata", getAllData)
	app.Post("/event/submit", postData)

	connectDB()

	listenString := fmt.Sprintf(":%d", DefaultPort)
	log.Fatal(app.Listen(listenString))
}
