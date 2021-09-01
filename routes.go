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
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func inviteToComp(c *gin.Context) {
	inviteID := c.Param("id")
	fromID := c.Query("inviteFrom")
	compID := c.Query("compID")

	if fromID == "" || compID == "" {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	sqlStatement := `INSERT INTO comp_reg (player_id, comp_id, invite_from, pending)
	VALUES ($1, $2, $3, true)`
	_, err := db.Exec(sqlStatement, inviteID, compID, fromID)
	if err != nil {
		println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusOK)

}

func getCompInvites(c *gin.Context) {
	playerID := c.Param("id")

	sqlStatement := `SELECT invite_from, first_name, last_name, comp_name, comp.id FROM comp_reg 
	LEFT JOIN comp ON comp.id = comp_reg.comp_id
	LEFT JOIN player on player.id = comp_reg.invite_from
	WHERE comp_reg.player_id = $1 AND pending=true;`

	rows, err := db.Query(sqlStatement, playerID)

	if err != nil {
		println(err.Error())
		if err == sql.ErrNoRows {
			c.Status(http.StatusNoContent)
			return
		} else {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	}

	invRes := InviteResponse{Invites: []Invite{}}
	for rows.Next() {
		var invite Invite
		err = rows.Scan(&invite.FromPlayer.Id, &invite.FromPlayer.FirstName, &invite.FromPlayer.LastName, &invite.CompName, &invite.CompID)
		if err != nil {
			println(err.Error())
		}
		invRes.Invites = append(invRes.Invites, invite)
	}

	c.JSON(http.StatusOK, invRes)

}

func updateCompInvite(c *gin.Context) {
	inviteID := c.Param("id")
	compID := c.Param("compid")
	acceptstr := c.Query("accept")

	if acceptstr == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	accept, _ := strconv.ParseBool(acceptstr)

	var err error
	var sqlStatement string

	if accept {
		sqlStatement = `UPDATE comp_reg SET reg_date=current_timestamp, pending=false where player_id=$1 AND comp_id=$2`
	} else {
		sqlStatement = `DELETE FROM comp_reg where player_id=$1 AND comp_id=$2 AND pending=true`
	}

	_, err = db.Exec(sqlStatement, inviteID, compID)
	if err != nil {
		println(err)
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusOK)

}

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

/// COMPETITIONS

func getCompWithID(c *gin.Context) {
	id := c.Param("id")

	var comp Competition
	sqlStatement := `SELECT id, comp_name, is_private FROM comp where id=$1;`

	err := db.QueryRow(sqlStatement, id).Scan(&comp.Id, &comp.Name, &comp.IsPrivate)
	if err != nil {
		handleError(err, c)
		return
	}

	c.JSON(http.StatusOK, comp)
}

func getPublicComps(c *gin.Context) {

	sqlStatement := `SELECT id, comp_name, is_private FROM comp where is_private=false;`

	getCompetitions(c, sqlStatement)

}

func getPlayerComps(c *gin.Context) {

	id := c.Param("id")

	sqlStatement := `SELECT id, comp_name, is_private FROM comp LEFT JOIN comp_reg ON comp.id = comp_reg.comp_id where comp_reg.player_id = $1;`

	getCompetitions(c, sqlStatement, id)

}

func getCompetitions(c *gin.Context, sqlStatement string, args ...interface{}) {

	rows, err := db.Query(sqlStatement, args...)

	if err != nil {
		handleError(err, c)
	}

	compResponse := CompetitionResponse{Competitions: []Competition{}}
	for rows.Next() {
		var compeition Competition
		err = rows.Scan(&compeition.Id, &compeition.Name, &compeition.IsPrivate)
		if err != nil {
			println(err.Error())
		}
		compResponse.Competitions = append(compResponse.Competitions, compeition)
	}

	c.JSON(http.StatusOK, compResponse)
}

func getCompPlayers(c *gin.Context) {
	id := c.Param("id")
	sqlStatement := `SELECT id, first_name, last_name FROM player 
	LEFT JOIN comp_reg ON id=comp_reg.player_id
	WHERE comp_reg.comp_id=$1;`
	queryPlayers(c, sqlStatement, id)
}

func getPlayers(c *gin.Context) {

	sqlStatement := `SELECT id, first_name, last_name FROM player;`
	queryPlayers(c, sqlStatement)
}

func queryPlayers(c *gin.Context, sqlStatement string, args ...interface{}) {

	rows, err := db.Query(sqlStatement, args...)
	if err != nil {
		handleError(err, c)
	}

	playersRes := PlayersResponse{Players: []Player{}}
	for rows.Next() {
		var player Player
		err = rows.Scan(&player.Id, &player.FirstName, &player.LastName)
		if err != nil {
			println(err.Error())
		}
		playersRes.Players = append(playersRes.Players, player)
	}

	c.JSON(http.StatusOK, playersRes)
}

func getPlayerWithID(c *gin.Context) {
	id := c.Param("id")

	var player Player
	sqlStatement := `SELECT id, first_name, last_name FROM player where id=$1;`
	err := db.QueryRow(sqlStatement, id).Scan(&player.Id, &player.FirstName, &player.LastName)
	if err != nil {
		handleError(err, c)
		return
	}

	c.JSON(http.StatusOK, player)
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
		handleError(err, c)
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
		handleError(err, c)
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
