package main

import (
	"./backend"
	"./config"
	"./database"
)

func main() {
	config.StartInit()
	db := database.Connect()

	defer database.DB.Close()
	backend.StartBack(db)
}
