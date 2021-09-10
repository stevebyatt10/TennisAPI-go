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
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// Endpoint: players/:id/invite
//
// Creates a new invite to competition for the specified players
func invitePlayersToComp(c *gin.Context) {
	CompID := c.Param("id")

	var request struct {
		FromID    int   `json:"fromID" binding:"required"`
		PlayerIDs []int `json:"playerIDs" binding:"required"`
	}

	if !tryGetRequest(c, &request) {
		return
	}

	for _, ID := range request.PlayerIDs {

		sqlStatement := `INSERT INTO comp_reg (player_id, comp_id, invite_from, pending)
		VALUES ($1, $2, $3, true)`
		_, err := db.Exec(sqlStatement, ID, CompID, request.FromID)
		if err != nil {
			println(err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
	}

	c.Status(http.StatusOK)

}

// Endpoint: /players/:id/invite
//
// Returns a list of all competition ivnites
func getCompInvites(c *gin.Context) {
	playerID := c.Param("id")

	sqlStatement := `SELECT invite_from, first_name, last_name, comp_name, comp.id, comp.is_private FROM comp_reg 
	LEFT JOIN comp ON comp.id = comp_reg.comp_id
	LEFT JOIN player on player.id = comp_reg.invite_from
	WHERE comp_reg.player_id = $1 AND pending=true;`

	rows, err := db.Query(sqlStatement, playerID)

	if handleError(err, c) {
		return
	}

	invRes := InviteResponse{Invites: []Invite{}}
	for rows.Next() {
		var invite Invite
		err = rows.Scan(&invite.FromPlayer.Id, &invite.FromPlayer.FirstName, &invite.FromPlayer.LastName, &invite.Comp.Name, &invite.Comp.Id, &invite.Comp.IsPrivate)
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
	var compDetails struct {
		CompName  string `form:"comp_name" binding:"required"`
		IsPrivate *bool  `form:"is_private" binding:"required"`
		CreatorId int    `form:"creator_id" binding:"required"`
	}
	if err := c.ShouldBind(&compDetails); err != nil {
		println(err.Error())
		c.Status(http.StatusBadRequest)
		return
	}

	sqlStatement := `INSERT INTO comp (comp_name, is_private, creator_id)
		VALUES ($1, $2, $3)
		RETURNING id`
	var comp Competition
	err := db.QueryRow(sqlStatement, compDetails.CompName, *compDetails.IsPrivate, compDetails.CreatorId).Scan(&comp.Id)
	if err != nil {
		println(err.Error())
		c.Status(http.StatusInternalServerError)
		return
	}

	err = joinComp(compDetails.CreatorId, *comp.Id)
	if err != nil {
		println(err.Error())
		c.Status(http.StatusInternalServerError)
		return
	}
	comp.Name = &compDetails.CompName
	comp.IsPrivate = compDetails.IsPrivate
	comp.CreatorID = &compDetails.CreatorId
	c.JSON(http.StatusCreated, comp)

}

// Endpoint: /matches/:id/score
//
// Updates the score for the match, creates new points, games or sets as necessary
// Returns a Score Object if game is still in progress, returns empty body when game finished
func scoreMatch(c *gin.Context) {

	param := c.Param("id")
	matchID, err := strconv.Atoi(param)
	if handleError(err, c) {
		return
	}

	var request struct {
		PointNum      int   `form:"pointNum" binding:"required"`
		Faults        int   `form:"faults"`
		Lets          int   `form:"lets"`
		Ace           *bool `form:"ace"`
		UnforcedError *bool `form:"unforcedError"`
		WinnerID      int   `form:"winnerID" binding:"required"`
	}

	var response ScoreResponse

	if !tryGetRequest(c, &request) {
		return
	}

	println("Updating current point")

	// Update the current point
	sqlStatement := `UPDATE point SET 
	faults=$3, 
	lets=$4,
	ace=$5,
	unforced_error=$6,
	winner_id=$7
	WHERE number=$1 AND match_id=$2`
	_, err = db.Exec(sqlStatement, request.PointNum, matchID, request.Faults, request.Lets, request.Ace, request.UnforcedError, request.WinnerID)
	if handleError(err, c) {
		return
	}

	println("Checking if match is over")

	// Get players wins
	sqlStatement = `
	SELECT player.id, (SELECT COUNT(winner_id) FROM point
			WHERE winner_id = player.id and match_id = $1) as wins
	FROM player
	JOIN match_participant mp ON player_id = player.id
	WHERE mp.match_id = $1;`
	rows, err := db.Query(sqlStatement, matchID)
	if handleError(err, c) {
		return
	}

	var winnerWins, otherWins int
	for rows.Next() {
		var wins, id int
		err = rows.Scan(&id, &wins)
		if err != nil {
			println(err.Error())
		}

		if id == request.WinnerID {
			winnerWins = wins
		} else {
			otherWins = wins
		}
	}

	// Get match info, points to win by, min points, server and receiver
	sqlStatement = `SELECT min_points, win_by, server_id, receiver_id FROM point
					JOIN match on point.match_id = match.id
					WHERE point.match_id = $1 and number = $2`

	var curServer, curReceiver, minpoints, winBy int
	err = db.QueryRow(sqlStatement, matchID, request.PointNum).Scan(&minpoints, &winBy, &curServer, &curReceiver)
	if handleError(err, c) {
		return
	}

	// Check if match over
	if winnerWins >= minpoints {
		dif := winnerWins - otherWins
		if dif >= winBy {
			// game over
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

			c.JSON(http.StatusOK, response)
			return

		}
	}

	// new point

	var newServer, newReceiver int
	// Swap server every 2 points
	if request.PointNum%2 == 0 {
		newServer = curReceiver
		newReceiver = curServer
	} else {
		newServer = curServer
		newReceiver = curReceiver
	}

	var newPointNum = request.PointNum + 1
	sqlStatement = `INSERT INTO point (number, match_id, server_id, receiver_id)
	VALUES ($1, $2, $3, $4)`
	_, err = db.Exec(sqlStatement, newPointNum, matchID, newServer, newReceiver)
	if handleError(err, c) {
		return
	}

	response = ScoreResponse{Point: &newPointNum, NewServer: &newServer, LastPointWinnerPts: winnerWins, OtherPlayerPoints: otherWins}

	c.JSON(http.StatusOK, response)

}

func newMatchInComp(c *gin.Context) {
	compID := c.Param("id")

	var request struct {
		StartDate  time.Time `form:"startDate" binding:"required"`
		ServerID   int       `form:"serverID" binding:"required"`
		ReceiverID int       `form:"receiverID" binding:"required"`
		NumPoints  int       `form:"numPoints" binding:"required"`
		WinBy      int       `form:"winBy"`
	}

	// Get query params into object
	if !tryGetRequest(c, &request) {
		return
	}

	// Create new match
	sqlStatement := `
		WITH new_match AS (INSERT INTO match (comp_id, start_date,  min_points, win_by) VALUES ($1, $2, $3, $4)
		RETURNING id)
		INSERT INTO match_participant
		VALUES
		( (SELECT id FROM new_match), $5),
		( (SELECT id FROM new_match), $6)
		RETURNING match_id
		`
	var match Match
	err := db.QueryRow(sqlStatement, compID, request.StartDate, request.NumPoints, request.WinBy, request.ServerID, request.ReceiverID).Scan(&match.MatchID)
	if handleError(err, c) {
		return
	}

	// Get players form their id's
	match.Player1, match.Player2 = getPlayersFromMatch(match.MatchID)

	var response struct {
		NewPoint ScoreResponse `json:"newPoint"`
		Match    Match         `json:"match"`
	}

	sqlStatement = `
	INSERT INTO point (number, match_id, server_id, receiver_id)
	VALUES (1, $1, $2, $3)`
	_, err = db.Exec(sqlStatement, match.MatchID, request.ServerID, request.ReceiverID)
	if handleError(err, c) {
		return
	}

	// FIXME: disgusting code
	res := ScoreResponse{NewServer: &request.ServerID, OtherPlayerPoints: 0, LastPointWinnerPts: 0}
	one := 1
	res.Point = &one
	response.NewPoint = res
	match.StartDate = &request.StartDate
	response.Match = match
	c.JSON(http.StatusOK, response)

}

// Creates a new set, game and point within the match
//
// Returns a Score Reponse containing the point number, game and set ID
// func newSetGamePoint(c *gin.Context, matchID, serverID, receiverID, setNumber, numPoints, numGames int) *ScoreResponse {

// 	var response ScoreResponse

// 	// Create a new set, game and point
// 	sqlStatement := `WITH new_set AS (
// 		INSERT INTO set (match_id, number, num_games)
// 		VALUES
// 		($1, $2, $3)
// 		RETURNING id
// 		),
// 		new_game AS (
// 		INSERT INTO game (set_id, number, server_id, receiver_id, num_points)
// 		VALUES
// 		( (SELECT id FROM new_set), 1, $4, $5, $6)
// 		RETURNING id
// 		),
// 		new_point AS (
// 		INSERT into point (number, game_id, server_id)
// 		VALUES
// 		(1,  (SELECT id FROM new_game), $4 )
// 		RETURNING number
// 		)
// 		SELECT new_point.number as pid, new_game.id as gid, new_set.id as sid
// 		FROM new_point, new_game, new_set`

// 	err := db.QueryRow(sqlStatement, matchID, setNumber, numGames, serverID, receiverID, numPoints).Scan(&response.Point, &response.Game, &response.Set)
// 	if handleError(err, c) {
// 		return nil
// 	}

// 	return &response
// }

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

	score := MatchScore{}
	score.Player1, score.Player2 = getMatchScore(match.MatchID, match.Player1.Id, match.Player2.Id)
	match.Score = &score

	if comp.Id != nil {
		match.Competition = &comp
	}

	c.JSON(http.StatusOK, match)
}

// Endpoint: /matches/:id/stats
//
// Get a count of all point stats for each player and stats for each point, game and set
func getMatchStats(c *gin.Context) {
	param := c.Param("id")
	matchID, err := strconv.Atoi(param)
	if handleError(err, c) {
		return
	}

	var response struct {
		Points  []Point          `json:"points"`
		Player1 PlayerMatchStats `json:"player1"`
		Player2 PlayerMatchStats `json:"player2"`
	}

	// Get player stats
	p1, p2 := getPlayersFromMatch(matchID)

	sqlStatement := `SELECT SUM(p.faults) as faults,
	Count(CASE WHEN p.faults>1 THEN 1 END ) as double_faults, 
	SUM(p.lets) as lets, 
	Count(CASE WHEN p.ace THEN 1 END) as aces,
	p.server_id 
	FROM point p
	LEFT JOIN match ON match.id = p.match_id
	where match.id = $1 AND (p.server_id = $2 OR p.server_id = $3)
	GROUP BY p.server_id`

	rows, err := db.Query(sqlStatement, matchID, p1.Id, p2.Id)
	if handleError(err, c) {
		return
	}

	for rows.Next() {
		var pstats PlayerMatchStats
		var id int
		err = rows.Scan(&pstats.Faults, &pstats.DoubleFaults, &pstats.Lets, &pstats.Aces, &id)
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

	// Count errors
	sqlStatement = `SELECT Count(CASE WHEN point.unforced_error THEN 1 END), player.id from player
	LEFT JOIN match_participant mp on mp.player_id = id
	LEFT JOIN point ON point.match_id = mp.match_id
	WHERE point.winner_id != player.id and mp.match_id = $1
	GROUP BY player.id;`

	rows, err = db.Query(sqlStatement, matchID)
	if handleError(err, c) {
		return
	}

	for rows.Next() {
		var errors, id int
		err = rows.Scan(&errors, &id)
		if err != nil {
			println(err.Error())
		}
		if p1.Id == id {
			response.Player1.Errors = errors
		} else {
			response.Player2.Errors = errors
		}
	}

	// Get all points from match
	sqlStatement = `SELECT number, winner_id, server_id, receiver_id, 
	faults, 
	CASE WHEN faults > 1 THEN TRUE 
	ELSE FALSE END double_fault, lets, ace, unforced_error 
	FROM point
	WHERE match_id = $1;`

	rows, err = db.Query(sqlStatement, matchID)
	if handleError(err, c) {
		return
	}

	for rows.Next() {
		var point Point
		err = rows.Scan(&point.Number, &point.WinnerID, &point.ServerID, &point.ReceiverID, &point.Stats.Faults, &point.Stats.DoubleFault, &point.Stats.Lets, &point.Stats.Ace, &point.Stats.Error)
		if err != nil {
			println(err.Error())
		}
		response.Points = append(response.Points, point)
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
	WHERE match.comp_id= $1 and match_result.winner_id is not null
	ORDER BY end_date DESC`
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

		score := MatchScore{}
		score.Player1, score.Player2 = getMatchScore(match.MatchID, match.Player1.Id, match.Player2.Id)
		match.Score = &score

		matchResponse.Matches = append(matchResponse.Matches, match)
	}

	c.JSON(http.StatusOK, matchResponse)
}

func getMatchScore(matchID, p1Id, p2Id int) (int, int) {

	// Get players wins
	sqlStatement := `
	SELECT player.id, (SELECT COUNT(winner_id) FROM point
			WHERE winner_id = player.id and match_id = $1) as wins
	FROM player
	JOIN match_participant mp ON player_id = player.id
	WHERE mp.match_id = $1;`
	rows, err := db.Query(sqlStatement, matchID)
	if err != nil {
		println(err.Error())
		return 0, 0
	}

	var p1wins, p2wins int
	for rows.Next() {
		var wins, id int
		err = rows.Scan(&id, &wins)
		if err != nil {
			println(err.Error())
		}

		if id == p1Id {
			p1wins = wins
		} else {
			p2wins = wins
		}
	}

	return p1wins, p2wins
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
	JOIN match ON match_id = match.id
	JOIN comp on match.comp_id = comp.id
	JOIN match_result mr ON mr.match_id =match.id
	where comp.id = $1 and player_id = p.id) AS played,  
	(SELECT count(winner_id)
	FROM match_result
	JOIN match ON match_id = match.id
	JOIN comp on match.comp_id = comp.id
	where comp.id = $1 and winner_id = p.id) AS wins  
	FROM player p
		JOIN match_participant mp on mp.player_id = p.id
		JOIN match m ON mp.match_id = m.id
	JOIN comp c ON c.id = m.comp_id
		JOIN match_result mr on mr.match_id = m.id 
	WHERE c.id = $1
	GROUP BY p.id
	ORDER BY  wins DESC
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

	sqlStatement := `SELECT id, comp_name, is_private, creator_id, COUNT(r.player_id), null as pos 
	FROM comp
	LEFT JOIN comp_reg r on id = r.comp_id
	WHERE is_private=false
	GROUP BY id;`

	res, err := getCompetitions(c, sqlStatement)
	if handleError(err, c) {
		return
	}
	c.JSON(http.StatusOK, res)

}

// Endpoint: /players/:id/comps
//
// Returns an array of comp objects that the player is registered in
func getPlayerComps(c *gin.Context) {

	param := c.Param("id")
	playerid, err := strconv.Atoi(param)
	if handleError(err, c) {
		return
	}

	sqlStatement := `SELECT id, comp_name, is_private, creator_id, (SELECT COUNT(player_id) FROM comp_reg WHERE comp_id = comp.id) as totalplayers, null as pos    
	FROM comp 
	LEFT JOIN comp_reg ON comp.id = comp_reg.comp_id 
	WHERE comp_reg.player_id = $1 and (pending=false or pending is null)`

	res, err := getCompetitions(c, sqlStatement, playerid)
	if handleError(err, c) {
		return
	}

	sqlStatement = `SELECT player.id, COUNT(player.id) as wins 
	FROM player
	JOIN match_result on id = match_result.winner_id
	JOIN match on match_result.match_id = match.id
	JOIN comp on match.comp_id = comp.id
	WHERE comp.id = $1
	group by player.id
	order by wins DESC
	;`

	// For each comp

	for index := 0; index < len(res.Competitions); index++ {
		rows, err := db.Query(sqlStatement, res.Competitions[index].Id)
		if handleError(err, c) {
			return
		}

		// For each player
		i := 1
		for rows.Next() {
			var id, wins int
			err = rows.Scan(&id, &wins)
			if err != nil {
				println(err.Error())
			}

			if id == playerid {
				res.Competitions[index].PlayerPos = &i
				break
			}

			i++

		}
	}

	c.JSON(http.StatusOK, res)

}

// Returns an array of competitions
//
// Handles error inside, status will reflect the success of the query
func getCompetitions(c *gin.Context, sqlStatement string, args ...interface{}) (*CompetitionResponse, error) {

	rows, err := db.Query(sqlStatement, args...)

	if err != nil {
		return nil, err
	}

	compResponse := CompetitionResponse{Competitions: []Competition{}}
	for rows.Next() {
		var comp Competition
		err = rows.Scan(&comp.Id, &comp.Name, &comp.IsPrivate, &comp.CreatorID, &comp.PlayerCount, &comp.PlayerPos)
		if err != nil {
			println(err.Error())
		}
		compResponse.Competitions = append(compResponse.Competitions, comp)
	}

	return &compResponse, nil
}

// Endpoint: /comps/:id/players
//
// Return an array of player objects within the specified comp
func getCompPlayers(c *gin.Context) {
	id := c.Param("id")
	sqlStatement := `SELECT id, first_name, last_name FROM player 
	LEFT JOIN comp_reg ON id=comp_reg.player_id
	WHERE comp_reg.comp_id=$1 and comp_reg.pending != true;`
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

	go sendWelcomeEmail(newPlayer)

}
