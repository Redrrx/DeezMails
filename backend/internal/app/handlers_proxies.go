package app

import (
	"net/http"

	"deezmails/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// createProxy godoc
// @Summary Create a proxy
// @Tags proxies
// @Accept json
// @Produce json
// @Param proxy body models.ProxyInput true "Proxy"
// @Success 201 {object} models.Proxy
// @Failure 400 {object} map[string]string
// @Router /api/proxies [post]
func (a *App) createProxy(c *gin.Context) {
	var input models.ProxyInput
	if err := bindJSON(c, &input); err != nil {
		badRequest(c, "invalid proxy input")
		return
	}
	p, err := input.Build()
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	p.Password, err = encryptCredential(a.credentials, p.Password, purposeProxyPassword)
	if err != nil {
		internalError(c, "could not encrypt proxy credentials", err)
		return
	}
	if err := a.requestDB(c).Create(&p).Error; err != nil {
		databaseWriteError(c, "could not create proxy", err)
		return
	}
	c.JSON(http.StatusCreated, p)
}

// listProxies godoc
// @Summary List proxies
// @Tags proxies
// @Produce json
// @Success 200 {array} models.Proxy
// @Router /api/proxies [get]
func (a *App) listProxies(c *gin.Context) {
	var items []models.Proxy
	if err := a.requestDB(c).Find(&items).Error; err != nil {
		internalError(c, "could not load proxies", err)
		return
	}
	c.JSON(http.StatusOK, items)
}

// updateProxy godoc
// @Summary Update a proxy
// @Tags proxies
// @Accept json
// @Produce json
// @Param id path int true "Proxy ID"
// @Param proxy body models.ProxyInput true "Proxy"
// @Success 200 {object} models.Proxy
// @Router /api/proxies/{id} [put]
func (a *App) updateProxy(c *gin.Context) {
	var p models.Proxy
	if err := a.requestDB(c).First(&p, c.Param("id")).Error; err != nil {
		handleLookupError(c, err)
		return
	}
	var input models.ProxyInput
	if bindJSON(c, &input) != nil {
		badRequest(c, "invalid proxy input")
		return
	}
	next, err := input.Build()
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	p.Name, p.Type, p.Host, p.Port, p.Username, p.URL = next.Name, next.Type, next.Host, next.Port, next.Username, next.URL
	if next.Password != "" {
		p.Password, err = encryptCredential(a.credentials, next.Password, purposeProxyPassword)
		if err != nil {
			internalError(c, "could not encrypt proxy credentials", err)
			return
		}
	}
	if err := a.requestDB(c).Save(&p).Error; err != nil {
		databaseWriteError(c, "could not update proxy", err)
		return
	}
	c.JSON(http.StatusOK, p)
}

// deleteProxy godoc
// @Summary Delete a proxy
// @Tags proxies
// @Param id path int true "Proxy ID"
// @Success 204
// @Router /api/proxies/{id} [delete]
func (a *App) deleteProxy(c *gin.Context) {
	var p models.Proxy
	if err := a.requestDB(c).First(&p, c.Param("id")).Error; err != nil {
		handleLookupError(c, err)
		return
	}
	if err := a.requestDB(c).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Account{}).Where("proxy_id = ?", p.ID).Update("proxy_id", nil).Error; err != nil {
			return err
		}
		return tx.Delete(&p).Error
	}); err != nil {
		internalError(c, "could not delete proxy", err)
		return
	}
	c.Status(http.StatusNoContent)
}
