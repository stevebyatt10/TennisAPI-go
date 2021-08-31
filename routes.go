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
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func joinComp(playerId int, compId int) error {
	sqlStatement := `INSERT INTO comp_reg (player_id, comp_id, reg_date)
	VALUES ($1, $2, current_timestamp)`
	_, err := db.Exec(sqlStatement, playerId, compId)
	return err
}

func createComp(c *gin.Context) {
	var compDetails CompCreateDetails

	if err := c.ShouldBind(&compDetails); err != nil {
		println(err.Error())
		c.Status(http.StatusBadRequest)
		return
	}

	sqlStatement := `INSERT INTO comp (comp_name, is_private, creator_id)
		VALUES ($1, $2, $3)
		RETURNING id`
	id := -1
	err := db.QueryRow(sqlStatement, compDetails.CompName, *compDetails.IsPrivate, compDetails.CreatorId).Scan(&id)
	if err != nil {
		println(err.Error())
		c.Status(http.StatusInternalServerError)
		return
	}

	err = joinComp(compDetails.CreatorId, id)
	if err != nil {
		println(err.Error())
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusCreated)

}

func getPlayers(c *gin.Context) {
	c.Status(http.StatusOK)
}

func login(c *gin.Context) {
	var loginDetails LoginDetails
	var err error

	// Get query params into object
	if err = c.ShouldBind(&loginDetails); err != nil {
		println(err.Error())
		c.Status(http.StatusBadRequest)
		return
	}

	var passwordHash string
	var id int
	sqlStatement := `SELECT id, password_hash FROM player WHERE email=LOWER($1) LIMIT 1;`
	err = db.QueryRow(sqlStatement, loginDetails.Email).Scan(&id, &passwordHash)
	if err != nil {
		println(err.Error())
		var status int
		if err == sql.ErrNoRows {
			status = http.StatusBadRequest
		} else {
			status = http.StatusInternalServerError
		}
		c.AbortWithStatus(status)
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(loginDetails.Password)) != nil {
		println("Incorrect password")

		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	token, err := CreateTokenInDB(id)
	if err != nil {
		println(err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Return token and user id
	retObj := PlayerToken{PlayerId: id, Token: token}

	c.JSON(http.StatusOK, retObj)
}

func logout(c *gin.Context) {
	token := c.GetHeader("Token")
	id := c.Query("id")
	sqlStatement := `DELETE FROM player_token WHERE player_id = $1 AND token = $2`
	_, err := db.Exec(sqlStatement, id, token)
	if err != nil {
		var status int
		if err.Error() == sql.ErrNoRows.Error() {
			status = http.StatusNotFound
		} else {
			status = http.StatusInternalServerError
		}
		println("error:", err.Error())
		c.AbortWithError(status, err)
		return
	}

	c.Status(http.StatusOK)
}

func registerPlayer(c *gin.Context) {
	var newPlayer PlayerRegister
	var err error

	// Get query params into object
	if err = c.ShouldBind(&newPlayer); err != nil {
		println(err.Error())
		c.Status(http.StatusBadRequest)
		return
	}

	// Check if email is in use
	var exists bool
	sqlStatement := `SELECT EXISTS(SELECT 1 FROM player WHERE email=LOWER($1));`
	err = db.QueryRow(sqlStatement, newPlayer.Email).Scan(&exists)
	if err != nil {
		println(err.Error())
		c.Status(http.StatusInternalServerError)
		return
	} else if exists {
		errorRes := ErrorResposne{Message: "Email already in use"}
		c.JSON(http.StatusConflict, errorRes)
		return
	}

	// Insert new player
	sqlStatement = `INSERT INTO player (first_name, last_name, email, password_hash, is_admin)
		VALUES ($1, $2, LOWER($3), $4, $5)
		RETURNING id`
	id := -1
	password := HashPassword(newPlayer.Password)
	err = db.QueryRow(sqlStatement, newPlayer.FirstName, newPlayer.LastName, newPlayer.Email, password, false).Scan(&id)
	if err != nil {
		println(err.Error())
		c.Status(http.StatusInternalServerError)
		return
	}
	fmt.Println("New record ID is:", id)

	token, err := CreateTokenInDB(id)
	if err != nil {
		println(err.Error())
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Return token and user id
	retObj := PlayerToken{PlayerId: id, Token: token}

	c.JSON(http.StatusCreated, retObj)
}
