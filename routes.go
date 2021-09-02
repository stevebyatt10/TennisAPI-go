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
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// Endpoint: players/:id/invite
//
// Creates a new invite to competition for the specified player
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

// Endpoint: /players/:id/invite
//
// Returns a list of all competition ivnites
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

// Endpoint: /players/:id/invite/compid
//
// If accepting invitation comp_reg is updated, pending = false
//
// If declining invite, comp_reg is deleted
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

// Helper function
//
// Adds the player to the comp
//
// Returns error object from queery
func joinComp(playerId int, compId int) error {
	sqlStatement := `INSERT INTO comp_reg (player_id, comp_id, reg_date)
	VALUES ($1, $2, current_timestamp)`
	_, err := db.Exec(sqlStatement, playerId, compId)
	return err
}

// Endpoint: /comps
//
// Cretes a new competition in the DB and returns the comp id
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
	var id int
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

// Endpoint: /matches/:id/score
//
// Updates the score for the match, creates new points, games or sets as necessary
//
// Returns a Score Object if game is still in progress, returns empty body when game finished
func scoreMatch(c *gin.Context) {

	param := c.Param("id")
	matchID, err := strconv.Atoi(param)
	if handleError(err, c) {
		return
	}

	var request struct {
		Point         int   `form:"pointNum" binding:"required"`
		Game          int   `form:"gameID" binding:"required"`
		Set           int   `form:"setID" binding:"required"`
		Faults        int   `form:"faults"`
		Lets          int   `form:"lets"`
		Ace           *bool `form:"ace"`
		UnforcedError *bool `form:"unforcedError"`
		WinnerID      int   `form:"winnerID" binding:"required"`
	}

	if !tryGetRequest(c, &request) {
		return
	}

	// Update the current point
	sqlStatement := `UPDATE point SET 
	faults=$3, 
	lets=$4,
	ace=$5,
	unforced_error=$6,
	winner_id=$7
	WHERE number=$1 AND game_id=$2`
	_, err = db.Exec(sqlStatement, request.Point, request.Game, request.Faults, request.Lets, request.Ace, request.UnforcedError, request.WinnerID)
	if handleError(err, c) {
		return
	}

	println("current point updated")

	// See if current game is over

	println("Checking if current game is over")

	var currGamePoints, currentServerID, maxGamePoints int
	sqlStatement = `SELECT server_id, num_points, 
	(SELECT COUNT(point.number) 
	FROM point 
	WHERE game_id = $1 AND point.winner_id = $2
	) AS cur_points 
	FROM game WHERE id = $1; `
	err = db.QueryRow(sqlStatement, request.Game, request.WinnerID).Scan(&currentServerID, &maxGamePoints, &currGamePoints)
	if handleError(err, c) {
		return
	}

	var response ScoreResponse

	// Game not over
	if currGamePoints < maxGamePoints {

		println("Current game still going, creating new point")
		// new point, same game
		// return new pointNum, same game ID and same set ID

		sqlStatement = `INSERT INTO point (number, game_id, server_id)
		VALUES
		($1, $2, $3)
		returning number
		`
		err := db.QueryRow(sqlStatement, request.Point+1, request.Game, currentServerID).Scan(&response.Point)
		if handleError(err, c) {
			return
		}
		response.Game = request.Game
		response.Set = request.Set
		c.JSON(http.StatusOK, response)
		return
	}

	// Game is over
	//
	// 	set game winner to point winner
	sqlStatement = `UPDATE game SET 
	winner_id=$1
	WHERE id=$2`
	_, err = db.Exec(sqlStatement, request.WinnerID, request.Game)
	if handleError(err, c) {
		return
	}

	println("Current game is over, updated game winner")

	// Check current set

	// 	see how many games winner has in set

	println("Checking if current set is over")

	var currGames, maxGames int
	sqlStatement = `SELECT num_games, 
	(SELECT COUNT(game.id) 
	FROM game
	WHERE set_id = $1 AND game.winner_id = $2
	) AS cur_games 
	FROM set WHERE id = $1; `
	err = db.QueryRow(sqlStatement, request.Set, request.WinnerID).Scan(&maxGames, &currGames)
	if handleError(err, c) {
		return
	}

	var newServer int
	// get alternating server
	sqlStatement = `SELECT player_id FROM match_participant
				WHERE match_id = $1 AND player_id != $2`
	err = db.QueryRow(sqlStatement, matchID, currentServerID).Scan(&newServer)
	if handleError(err, c) {
		return
	}

	// Set not over
	if currGames < maxGames {

		println("Current set is not over")

		// new game under set, set server to other player
		sqlStatement = `WITH ret_new_game AS 
		(
		WITH new_game AS (SELECT
		set.id as new_id,
		set.number+1 as new_number, 
		num_points
		FROM set 
		LEFT JOIN match ON match_id = match.id
		LEFT JOIN game ON set_id = game.set_id
		WHERE set.id = $1
		LIMIT 1)
		INSERT INTO game (set_id, number, server_id, receiver_id, num_points)
		VALUES
		( 
		(SELECT new_id FROM new_game),
		(SELECT new_number FROM new_game),
		$2,
		$3,
		(SELECT num_points FROM new_game) 
		)
		RETURNING id)
		INSERT INTO POINT (number, game_id, server_id)
		VALUES
		(1, ( SELECT id FROM ret_new_game ), $2 )
		RETURNING number, game_id
		;`
		// return new pointNum, new game ID and same set ID

		println("Created new game and new point")

		err := db.QueryRow(sqlStatement, request.Set, newServer, currentServerID).Scan(&response.Point, &response.Game)
		if handleError(err, c) {
			return
		}
		response.Set = request.Set
		c.JSON(http.StatusOK, response)
		return
	}

	println("Current set is over, update set winner")

	// // 	if set is over
	// set set winner to point winner
	sqlStatement = `UPDATE set SET 
		winner_id=$1
		WHERE id=$2`
	_, err = db.Exec(sqlStatement, request.WinnerID, request.Set)
	if handleError(err, c) {
		return
	}

	// 	see how many sets winner has in match

	println("Checking if current match is over")

	var numSets, currSets int
	sqlStatement = `SELECT num_sets,
	(SELECT COUNT(set.id)
	FROM set
	WHERE match_id = $1 AND set.winner_id = $2
	) AS curr_sets
	FROM match WHERE id = $1; `
	err = db.QueryRow(sqlStatement, matchID, request.WinnerID).Scan(&numSets, &currSets)
	if handleError(err, c) {
		return
	}

	// if match not over
	if currSets < numSets {

		println("Current match is not over")

		println("Get alternating server")

		// Create new set, game, point
		res := newSetGamePoint(c, matchID, newServer, currentServerID, request.Set+1, maxGamePoints, maxGames)
		if res == nil {
			return
		}

		println("Created new set, game and point")

		response = *res
		c.JSON(http.StatusOK, response)
		return
	}

	// if match  over

	println("Current match over creating new result")

	// create new match result with winner
	sqlStatement = `INSERT INTO match_result (match_id, winner_id)
	VALUES
	($1, $2)`
	_, err = db.Exec(sqlStatement, matchID, request.WinnerID)
	if handleError(err, c) {
		return
	}

	// Update match for end date
	sqlStatement = `UPDATE match SET end_date=current_timestamp WHERE id = $1`
	_, err = db.Exec(sqlStatement, matchID)
	if handleError(err, c) {
		return
	}

	c.Status(http.StatusOK)

}

func newMatchInComp(c *gin.Context) {
	compID := c.Param("id")

	var request struct {
		Player1ID  int       `form:"player1ID" binding:"required"`
		Player2ID  int       `form:"player2ID" binding:"required"`
		StartDate  time.Time `form:"startDate" binding:"required"`
		ServerID   int       `form:"serverID" binding:"required"`
		ReceiverID int       `form:"receiverID" binding:"required"`
		NumSets    int       `form:"numSets" binding:"required"`
		NumGames   int       `form:"numGames" binding:"required"`
		NumPoints  int       `form:"numPoints" binding:"required"`
	}

	// Get query params into object
	if !tryGetRequest(c, &request) {
		return
	}

	// Create new match
	sqlStatement := `
		WITH new_match AS (INSERT INTO match (comp_id, start_date,  num_sets) VALUES ($1, $2, $3)
		RETURNING id)
		INSERT INTO match_participant
		VALUES
		( (SELECT id FROM new_match), $4),
		( (SELECT id FROM new_match), $5)
		RETURNING match_id
		`

	var match Match
	err := db.QueryRow(sqlStatement, compID, request.StartDate, request.NumSets, request.Player1ID, request.Player2ID).Scan(&match.MatchID)
	if handleError(err, c) {
		return
	}

	// Get players form their id's
	match.Player1, match.Player2 = getPlayersFromMatch(match.MatchID)

	var response struct {
		NewIDs ScoreResponse `json:"newIDs"`
		Match  Match         `json:"match"`
	}

	res := newSetGamePoint(c, match.MatchID, request.ServerID, request.ReceiverID, 1, request.NumPoints, request.NumGames)
	if res == nil {
		return
	}

	response.NewIDs = *res
	match.StartDate = &request.StartDate
	response.Match = match
	c.JSON(http.StatusOK, response)

}

// Creates a new set, game and point within the match
//
// Returns a Score Reponse containing the point number, game and set ID
func newSetGamePoint(c *gin.Context, matchID, serverID, receiverID, setNumber, numPoints, numGames int) *ScoreResponse {

	var response ScoreResponse

	// Create a new set, game and point
	sqlStatement := `WITH new_set AS (
		INSERT INTO set (match_id, number, num_games)
		VALUES
		($1, $2, $3)
		RETURNING id
		), 
		new_game AS (
		INSERT INTO game (set_id, number, server_id, receiver_id, num_points)
		VALUES
		( (SELECT id FROM new_set), 1, $4, $5, $6)
		RETURNING id
		),
		new_point AS (
		INSERT into point (number, game_id, server_id)
		VALUES
		(1,  (SELECT id FROM new_game), $7 )
		RETURNING number
		)
		SELECT new_point.number as pid, new_game.id as gid, new_set.id as sid
		FROM new_point, new_game, new_set`

	err := db.QueryRow(sqlStatement, matchID, setNumber, numGames, serverID, receiverID, numPoints, receiverID).Scan(&response.Point, &response.Game, &response.Set)
	if handleError(err, c) {
		return nil
	}

	return &response
}

// Endpoint /matches/:id
//
// Returns a match object from the provided endpoint
func getMatchFromID(c *gin.Context) {
	matchID := c.Param("id")

	sqlStatement := `SELECT match.id, match.comp_id, comp.comp_name, comp.is_private, start_date, end_date, winner_id 
		FROM match
		LEFT JOIN match_result ON match.id = match_result.match_id
		LEFT JOIN comp ON comp.id = match.comp_id
		WHERE match.id = $1`

	var match Match
	var comp Competition

	err := db.QueryRow(sqlStatement, matchID).Scan(&match.MatchID, &comp.Id, &comp.Name, &comp.IsPrivate, &match.StartDate,
		&match.EndDate, &match.WinnerID)

	if handleError(err, c) {
		return
	}

	match.Player1, match.Player2 = getPlayersFromMatch(match.MatchID)

	if comp.Id != nil {
		match.Competition = &comp
	}

	c.JSON(http.StatusOK, match)
}

// Endpoint: /matches/:id/stats
//
// Get a count of all point stats for each player
func getMatchStats(c *gin.Context) {
	param := c.Param("id")
	matchID, err := strconv.Atoi(param)
	if handleError(err, c) {
		return
	}

	var response struct {
		Player1 PointStats `json:"player1"`
		Player2 PointStats `json:"player2"`
	}

	p1, p2 := getPlayersFromMatch(matchID)
	println(matchID, p1.Id, p2.Id)

	sqlStatement := `SELECT SUM(p.faults) as faults,
	Count(CASE WHEN p.faults>1 THEN 1 END ) as double_faults, 
	SUM(p.lets) as lets, 
	Count(CASE WHEN p.ace THEN 1 END) as aces,
	Count(CASE WHEN p.unforced_error THEN 1 END) as errors,
	game.server_id 
	FROM point p
	LEFT JOIN game ON game_id = game.id
	LEFT JOIN set ON game.set_id = set.id
	LEFT JOIN match ON match.id = set.match_id
	where match.id = $1 AND (game.server_id = $2 OR game.server_id = $3)
	GROUP BY game.server_id`

	rows, err := db.Query(sqlStatement, matchID, p1.Id, p2.Id)
	if handleError(err, c) {
		return
	}

	for rows.Next() {
		var pstats PointStats
		var id int
		err = rows.Scan(&pstats.Faults, &pstats.DoubleFaults, &pstats.Lets, &pstats.Aces, &pstats.Errors, &id)
		if err != nil {
			println(err.Error())
		}
		if p1.Id == id {
			pstats.Player = p1
			response.Player1 = pstats
		} else {
			pstats.Player = p2
			response.Player2 = pstats
		}
	}

	c.JSON(http.StatusOK, response)

}

// Endpoint: /comps/:id/matches
//
// Return all matches within the comp
func getCompMatches(c *gin.Context) {
	compID := c.Param("id")

	sqlStatement := `SELECT id, start_date, end_date, winner_id FROM match
	LEFT JOIN match_result ON match.id = match_result.match_id
	WHERE match.comp_id= $1`
	rows, err := db.Query(sqlStatement, compID)

	if handleError(err, c) {
		return
	}

	var matchResponse struct {
		Matches []Match `json:"matches"`
	}
	matchResponse.Matches = []Match{}
	for rows.Next() {
		var match Match
		err = rows.Scan(&match.MatchID, &match.StartDate, &match.EndDate, &match.WinnerID)
		if err != nil {
			println(err.Error())
		}

		match.Player1, match.Player2 = getPlayersFromMatch(match.MatchID)

		matchResponse.Matches = append(matchResponse.Matches, match)
	}

	c.JSON(http.StatusOK, matchResponse)
}

// Returns 2 pointers to each player in the specified match
func getPlayersFromMatch(matchID int) (*Player, *Player) {
	var player1, player2 *Player

	pStatement := `SELECT id, first_name, last_name FROM player 
	LEFT JOIN match_participant mp ON mp.player_id = player.id
	WHERE match_id = $1`
	prows, perr := db.Query(pStatement, matchID)
	if perr != nil {
		println(perr.Error())
	}

	p := true
	for prows.Next() {
		var scannedPlayer Player
		perr = prows.Scan(&scannedPlayer.Id, &scannedPlayer.FirstName, &scannedPlayer.LastName)
		if perr != nil {
			println(perr.Error())
		}

		if p {
			player1 = &scannedPlayer
		} else {
			player2 = &scannedPlayer
		}
		p = !p
	}
	return player1, player2
}

// Endpoint: /comps/:id
//
// Return comp object from comp ID
func getCompWithID(c *gin.Context) {
	id := c.Param("id")

	var comp Competition
	sqlStatement := `SELECT id, comp_name, is_private FROM comp where id=$1;`

	err := db.QueryRow(sqlStatement, id).Scan(&comp.Id, &comp.Name, &comp.IsPrivate)
	if handleError(err, c) {
		return
	}

	c.JSON(http.StatusOK, comp)
}

// Endpoint: /comps/:id/table
//
// Return an array of table rows containing data about each competitor
func getCompTable(c *gin.Context) {
	id := c.Param("id")

	type Competitor struct {
		Player Player `json:"player"`
		Played int    `json:"played"`
		Wins   int    `json:"wins"`
		Losses int    `json:"losses"`
	}

	var response struct {
		Competitors []Competitor `json:"competitors"`
	}

	sqlStatement := `SELECT 
	p.id, first_name, last_name,
	(SELECT count(player_id)
	FROM match_participant
	LEFT JOIN match ON match_id = match.id
	LEFT JOIN comp on match.comp_id = comp.id
	JOIN match_result mr ON mr.match_id =match.id
	where comp.id = $1 and player_id = p.id) AS played,  
	COUNT(winner_id) as wins 
	FROM player p
	LEFT OUTER JOIN match_result mr ON mr.winner_id = p.id
	LEFT JOIN match m ON mr.match_id = m.id
	LEFT JOIN comp c ON c.id = m.comp_id
	WHERE c.id = $1
	GROUP BY p.id
	ORDER BY count(winner_id) DESC, p.id
	`

	rows, err := db.Query(sqlStatement, id)
	if handleError(err, c) {
		return
	}

	response.Competitors = []Competitor{}
	for rows.Next() {
		var competitor Competitor
		err = rows.Scan(&competitor.Player.Id, &competitor.Player.FirstName, &competitor.Player.LastName, &competitor.Played, &competitor.Wins)
		if err != nil {
			println(err.Error())
		}

		competitor.Losses = competitor.Played - competitor.Wins
		response.Competitors = append(response.Competitors, competitor)
	}

	c.JSON(http.StatusOK, response)
}

// Endpoint: /comps
//
// Returns an array of comp objects, only comps that are public
func getPublicComps(c *gin.Context) {

	sqlStatement := `SELECT id, comp_name, is_private FROM comp where is_private=false;`

	getCompetitions(c, sqlStatement)

}

// Endpoint: /players/:id/comps
//
// Returns an array of comp objects that the player is registered in
func getPlayerComps(c *gin.Context) {

	id := c.Param("id")

	sqlStatement := `SELECT id, comp_name, is_private FROM comp LEFT JOIN comp_reg ON comp.id = comp_reg.comp_id where comp_reg.player_id = $1;`

	getCompetitions(c, sqlStatement, id)

}

// Returns an array of competitions
//
// Handles error inside, status will reflect the success of the query
func getCompetitions(c *gin.Context, sqlStatement string, args ...interface{}) {

	rows, err := db.Query(sqlStatement, args...)

	if handleError(err, c) {
		return
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

// Endpoint: /comps/:id/players
//
// Return an array of player objects within the specified comp
func getCompPlayers(c *gin.Context) {
	id := c.Param("id")
	sqlStatement := `SELECT id, first_name, last_name FROM player 
	LEFT JOIN comp_reg ON id=comp_reg.player_id
	WHERE comp_reg.comp_id=$1;`
	queryPlayers(c, sqlStatement, id)
}

// Endpoint: /players
//
// Return an array of all player objects
func getPlayers(c *gin.Context) {

	sqlStatement := `SELECT id, first_name, last_name FROM player;`
	queryPlayers(c, sqlStatement)
}

// Helper function
//
// For returning an array of players with the same fields,
// Provide a special query and args for query
func queryPlayers(c *gin.Context, sqlStatement string, args ...interface{}) {

	rows, err := db.Query(sqlStatement, args...)
	if handleError(err, c) {
		return
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

// Endpoint: /player/:id
//
// Returns a player object from the specified ID
func getPlayerWithID(c *gin.Context) {
	id := c.Param("id")

	var player Player
	sqlStatement := `SELECT id, first_name, last_name FROM player where id=$1;`
	err := db.QueryRow(sqlStatement, id).Scan(&player.Id, &player.FirstName, &player.LastName)
	if handleError(err, c) {
		return
	}

	c.JSON(http.StatusOK, player)
}

// Endpoint: /login
//
// If email and password match a record in the DB a new token is created
// The player ID and token is returned
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
	if handleError(err, c) {
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

// Endpoint: /logout
//
// Deletes the token from the database to prevent further use
func logout(c *gin.Context) {
	token := c.GetHeader("Token")
	id := c.Query("id")
	sqlStatement := `DELETE FROM player_token WHERE player_id = $1 AND token = $2`
	_, err := db.Exec(sqlStatement, id, token)
	if handleError(err, c) {
		return
	}
	c.Status(http.StatusOK)
}

// Endpoint: /register
//
// Creates a new player in the database if email does not already exist
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
