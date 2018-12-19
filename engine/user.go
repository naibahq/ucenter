package engine

import (
	"net/http"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/naiba/com"

	"git.cm/naiba/ucenter"
	"git.cm/naiba/ucenter/pkg/nbgin"
	"github.com/gin-gonic/gin"
	"github.com/mssola/user_agent"
	"golang.org/x/crypto/bcrypt"
	"gopkg.in/go-playground/validator.v9"
)

func login(c *gin.Context) {
	c.HTML(http.StatusOK, "page/login", gin.H{})
}

func loginHandler(c *gin.Context) {
	type loginForm struct {
		Username string `form:"username" cfn:"用户名" binding:"required,min=2,max=12"`
		Password string `form:"password" cfn:"密码" binding:"required,min=6,max=32"`
	}
	var lf loginForm
	var u ucenter.User
	var errors validator.ValidationErrorsTranslations

	// 验证用户输入
	if err := c.ShouldBind(&lf); err != nil {
		errors = err.(validator.ValidationErrors).Translate(ucenter.ValidatorTrans)
	} else if err = ucenter.DB.Where("username = ?", lf.Username).First(&u).Error; err != nil {
		errors = map[string]string{
			"loginForm.用户名": "用户不存在",
		}
	} else if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(lf.Password)) != nil {
		errors = map[string]string{
			"loginForm.密码": "密码不正确",
		}
	}

	if errors != nil {
		c.HTML(http.StatusOK, "page/login", gin.H{
			"errors": map[string]interface{}{
				"Username": errors["loginForm.用户名"],
				"Password": errors["loginForm.密码"],
			},
		})
		return
	}

	rawUA := c.Request.UserAgent()
	ua := user_agent.New(rawUA)
	var loginClient ucenter.LoginClient
	loginClient.UserID = u.ID
	loginClient.Token = com.MD5(rawUA + time.Now().String() + u.Username)
	browser, _ := ua.Browser()
	loginClient.Name = ua.OS() + " " + browser
	loginClient.IP = c.ClientIP()
	loginClient.Expire = time.Now().Add(ucenter.AuthCookieExpiretion)
	if err := ucenter.DB.Save(&loginClient).Error; err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	nbgin.SetCookie(c, ucenter.AuthCookieName, loginClient.Token)
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	if from := c.Query("from"); strings.HasPrefix(from, "/") {
		c.Redirect(http.StatusFound, from)
		return
	}
	c.String(http.StatusOK, "登录成功")
}

func signup(c *gin.Context) {
	c.HTML(http.StatusOK, "page/signup", gin.H{})
}

func signupHandler(c *gin.Context) {
	type signUpForm struct {
		Username   string `form:"username" cfn:"用户名" binding:"required,min=2,max=12"`
		Password   string `form:"password" cfn:"密码" binding:"required,min=6,max=32"`
		RePassword string `form:"repassword" cfn:"确认密码" binding:"required,min=6,max=32,eqfield=Password"`
	}
	var suf signUpForm
	var u ucenter.User
	var errors validator.ValidationErrorsTranslations
	if err := c.ShouldBind(&suf); err != nil {
		errors = err.(validator.ValidationErrors).Translate(ucenter.ValidatorTrans)
	} else if err = ucenter.DB.Where("username = ?", suf.Username).First(&u).Error; err != gorm.ErrRecordNotFound {
		errors = map[string]string{
			"signUpForm.用户名": "用户名已存在",
		}
	}
	if errors != nil {
		c.HTML(http.StatusOK, "page/signup", gin.H{
			"errors": map[string]interface{}{
				"Username":   errors["signUpForm.用户名"],
				"Password":   errors["signUpForm.密码"],
				"RePassword": errors["signUpForm.确认密码"],
			},
		})
		return
	}
	u.Username = suf.Username
	bPass, err := bcrypt.GenerateFromPassword([]byte(suf.Password), bcrypt.DefaultCost)
	if err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	u.Password = string(bPass)
	if err := ucenter.DB.Save(&u).Error; err != nil {
		c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
	c.Redirect(http.StatusFound, "/login?"+c.Request.URL.RawQuery)
}