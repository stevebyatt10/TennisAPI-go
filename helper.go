// Copyright 2021 Stephen
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func connectToDB() {

	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	var err error
	db, err = sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}

	if err = db.Ping(); err != nil {
		panic(err)
	}

	fmt.Println("Successfully connected to database")

}

func CreateTokenInDB(playerId int) (string, error) {
	// Generate a token
	token := GenerateSecureToken(20)
	fmt.Println("Token is:", token)

	// add token to db
	sqlStatement := `INSERT INTO player_token (player_id, token) VALUES ($1, $2)`
	_, err := db.Exec(sqlStatement, playerId, token)
	if err != nil {
		return "", err
	}

	return token, nil
}

func GenerateSecureToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

func HashPassword(password string) string {
	passwordBytes := []byte(password)

	hash, err := bcrypt.GenerateFromPassword(passwordBytes, bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return string(hash)
}

func handleError(err error, c *gin.Context) {
	var status int
	if err == sql.ErrNoRows {
		status = http.StatusNotFound
	} else {
		status = http.StatusInternalServerError
	}
	println(err.Error())
	c.AbortWithError(status, err)
}
