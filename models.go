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

import "time"

type PlayerRegister struct {
	FirstName string `form:"first_name" binding:"required"`
	LastName  string `form:"last_name"`
	Email     string `form:"email" binding:"required"`
	Password  string `form:"password" binding:"required"`
}

type LoginDetails struct {
	Email    string `form:"email" binding:"required"`
	Password string `form:"password" binding:"required"`
}

type PlayerToken struct {
	PlayerId int    `json:"player_id"`
	Token    string `json:"token"`
}

type ErrorResposne struct {
	Message string `json:"error"`
}

type Competition struct {
	Id        *int    `json:"id"`
	Name      *string `json:"name"`
	IsPrivate *bool   `json:"isPrivate"`
	CreatorID *int    `json:"creatorID"`
}

type CompetitionResponse struct {
	Competitions []Competition `json:"competitions"`
}

type InviteResponse struct {
	Invites []Invite `json:"invites"`
}
type Invite struct {
	Comp       Competition `json:"comp"`
	FromPlayer Player      `json:"fromPlayer"`
}

type Player struct {
	Id        int    `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

type PlayersResponse struct {
	Players []Player `json:"players"`
}

type Match struct {
	MatchID     int          `json:"matchID"`
	Competition *Competition `json:"competition"`
	Player1     *Player      `json:"player1"`
	Player2     *Player      `json:"player2"`
	StartDate   *time.Time   `json:"startDate"`
	EndDate     *time.Time   `json:"endDate"`
	WinnerID    *int         `json:"winnerID"`
}

type ScoreResponse struct {
	Point int `json:"pointNum"`
	Game  int `json:"gameID"`
	Set   int `json:"setID"`
}

type PlayerMatchStats struct {
	Player       *Player `json:"player"`
	Faults       int     `json:"faults"`
	DoubleFaults int     `json:"doubleFaults"`
	Lets         int     `json:"lets"`
	Aces         int     `json:"aces"`
	Errors       int     `json:"errors"`
}

type Point struct {
	Number     int        `json:"number"`
	WinnerID   int        `json:"winnerID"`
	ServerID   int        `json:"serverID"`
	ReceiverID int        `json:"receiverID"`
	Stats      PointStats `json:"stats"`
}

type PointStats struct {
	Faults      int  `json:"faults"`
	DoubleFault bool `json:"doubleFault"`
	Lets        int  `json:"lets"`
	Ace         bool `json:"ace"`
	Error       bool `json:"error"`
}

type Game struct {
	WinnerID int     `json:"winnerID"`
	GameID   int     `json:"id"`
	Number   int     `json:"number"`
	Points   []Point `json:"points"`
}

type Set struct {
	WinnerID int    `json:"winnerID"`
	SetID    int    `json:"id"`
	Number   int    `json:"number"`
	Games    []Game `json:"games"`
}
