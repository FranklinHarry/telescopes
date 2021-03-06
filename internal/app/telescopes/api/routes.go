// Copyright © 2018 Banzai Cloud
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

package api

import (
	"fmt"
	"net/http"
	"os"

	"github.com/banzaicloud/bank-vaults/auth"
	"github.com/banzaicloud/telescopes/pkg/recommender"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	log "github.com/sirupsen/logrus"
	"gopkg.in/go-playground/validator.v8"
)

const (
	providerParam = "provider"
	regionParam   = "region"
)

// RouteHandler struct that wraps the recommender engine
type RouteHandler struct {
	engine *recommender.Engine
}

// NewRouteHandler creates a new RouteHandler and returns a reference to it
func NewRouteHandler(e *recommender.Engine) *RouteHandler {
	return &RouteHandler{
		engine: e,
	}
}

func getCorsConfig() cors.Config {
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	if !config.AllowAllOrigins {
		config.AllowOrigins = []string{"http://", "https://"}
	}
	config.AllowMethods = []string{http.MethodPut, http.MethodDelete, http.MethodGet, http.MethodPost, http.MethodOptions}
	config.AllowHeaders = []string{"Origin", "Authorization", "Content-Type"}
	config.ExposeHeaders = []string{"Content-Length"}
	config.AllowCredentials = true
	config.MaxAge = 12
	return config
}

// ConfigureRoutes configures the gin engine, defines the rest API for this application
func (r *RouteHandler) ConfigureRoutes(router *gin.Engine) {
	log.Info("configuring routes")

	v := binding.Validator.Engine().(*validator.Validate)

	basePath := "/"
	if basePathFromEnv := os.Getenv("TELESCOPES_BASEPATH"); basePathFromEnv != "" {
		basePath = basePathFromEnv
	}

	router.Use(cors.New(getCorsConfig()))

	base := router.Group(basePath)
	{
		base.GET("/status", r.signalStatus)
	}

	v1 := base.Group("/api/v1")
	v1.Use(ValidatePathParam(providerParam, v, "provider"))
	v1.Use(ValidateRegionData(v))
	recGroup := v1.Group("/recommender")
	{
		recGroup.POST("/:provider/:region/cluster/", r.recommendClusterSetup)
	}
}

// EnableAuth enables authentication middleware
func (r *RouteHandler) EnableAuth(router *gin.Engine, role string, sgnKey string) {
	router.Use(auth.JWTAuth(auth.NewVaultTokenStore(role), sgnKey, nil))
}

func (r *RouteHandler) signalStatus(c *gin.Context) {
	c.JSON(http.StatusOK, "ok")
}

// swagger:route POST /recommender/:provider/:region/cluster recommend recommendClusterSetup
//
// Provides a recommended set of node pools on a given provider in a specific region.
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
//     Schemes: http
//
//     Security:
//
//     Responses:
//       200: RecommendationResponse
func (r *RouteHandler) recommendClusterSetup(c *gin.Context) {
	log.Info("recommend cluster setup")
	provider := c.Param(providerParam)
	region := c.Param(regionParam)

	// request decorated with provider and region
	req := RequestWrapper{Provider: provider, Region: region}

	if err := c.BindJSON(&req); err != nil {
		log.Errorf("failed to bind request body: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "bad_params",
			"message": "validation failed",
			"cause":   err.Error(),
		})
		return
	}

	if response, err := r.engine.RecommendCluster(provider, region, req.ClusterRecommendationReq); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"status": http.StatusInternalServerError, "message": fmt.Sprintf("%s", err)})
	} else {
		c.JSON(http.StatusOK, *response)
	}
}

// RequestWrapper internal struct for passing provider/zone info to the validator
type RequestWrapper struct {
	recommender.ClusterRecommendationReq
	Provider string
	Region   string
}
