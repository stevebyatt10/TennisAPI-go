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
	connectToDB()
	router := gin.Default()
	router.Use(CORSMiddleware())

	router.GET("/players", ensureAuthenticated(), getPlayers)
	router.POST("/register", registerPlayer)
	router.POST("/login", login)
	router.POST("/logout", ensureAuthenticated(), logout)

	router.POST("/comps", ensureAuthenticated(), createComp)

	router.Run(":8080")
}
