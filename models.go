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

type CompCreateDetails struct {
	CompName  string `form:"comp_name" binding:"required"`
	IsPrivate *bool  `form:"is_private" binding:"required"`
	CreatorId int    `form:"creator_id" binding:"required"`
}

type ErrorResposne struct {
	Message string `json:"error"`
}
