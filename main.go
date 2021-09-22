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
	_ "net/http"

	_ "golang.org/x/crypto/bcrypt"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
)

var db *sql.DB

func main() {
	// gin.SetMode(gin.ReleaseMode)
	connectToDB()
	router := gin.Default()
	router.Use(CORSMiddleware())

	router.POST("/register", registerPlayer)
	router.POST("/login", login)
	router.POST("/logout", ensureAuthenticated(), logout)

	playersGroup := router.Group("/players")
	{
		playersGroup.Use(ensureAuthenticated())

		playersGroup.GET("", getPlayers)
		playersGroup.GET("/:id", getPlayerWithID)

		playersGroup.GET("/:id/comps", getPlayerComps)
		playersGroup.GET("/:id/invite", getCompInvites)
		playersGroup.PUT("/:id/invite/:compid", updateCompInvite)

	}

	matchesGroup := router.Group("/matches")
	{
		matchesGroup.Use(ensureAuthenticated())

		matchesGroup.GET("/:id", getMatchFromID)
		matchesGroup.POST("/:id/score", scoreMatch)
		matchesGroup.GET("/:id/stats", getMatchStats)
		matchesGroup.GET("/:id/latest", getMatchLatestPoint)
		matchesGroup.DELETE("/:id/latest", deleteLatestPoint)

	}

	compsGroup := router.Group("/comps")
	{
		compsGroup.Use(ensureAuthenticated())

		compsGroup.POST("", createComp)
		compsGroup.GET("", getPublicComps)

		compIdGroup := compsGroup.Group("/:id")
		{
			compIdGroup.GET("", getCompWithID)

			compIdGroup.GET("/players", getCompPlayers)

			compIdGroup.GET("/matches", getCompMatches)
			compIdGroup.POST("/matches", newMatchInComp)

			compIdGroup.POST("/invite", invitePlayersToComp)

			compIdGroup.GET("/table", getCompTable)
		}

	}

	router.Run(":8080")
}
