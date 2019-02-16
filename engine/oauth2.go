package engine

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"

	"github.com/lib/pq"

	"github.com/naiba/ucenter/pkg/fosite-storage"

	"github.com/naiba/ucenter"
	"github.com/naiba/ucenter/pkg/nbgin"
	"github.com/ory/fosite"

	"github.com/RangelReale/osin"
	"github.com/gin-gonic/gin"
)

func introspectionEndpoint(c *gin.Context) {

}

func revokeEndpoint(c *gin.Context) {

}

func oauth2auth(c *gin.Context) {
	ctx := fosite.NewContext()
	// Let's create an AuthorizeRequest object!
	// It will analyze the request and extract important information like scopes, response type and others.
	ar, err := oauth2provider.NewAuthorizeRequest(ctx, c.Request)
	if err != nil {
		log.Printf("Error occurred in NewAuthorizeRequest: %+v", err)
		oauth2provider.WriteAuthorizeError(c.Writer, ar, err)
		return
	}

	// Normally, this would be the place where you would check if the user is logged in and gives his consent.
	// We're simplifying things and just checking if the request includes a valid username and password
	user, ok := c.Get(ucenter.AuthUser)
	if ok {
		user := user.(*ucenter.User)
		ucenter.DB.Model(user).Where("client_id = ?", ar.GetClient().GetID()).Association("UserAuthorizeds").Find(&user.UserAuthorizeds)
		if c.Request.Method == http.MethodGet {
			if len(user.UserAuthorizeds) != 1 || storage.IsArgEqual(ar.GetRequestedScopes(), fosite.Arguments(user.UserAuthorizeds[0].Scope)) {
				// 需要用户授予权限
				var checkPerms = make(map[string]bool)
				for _, scope := range ar.GetRequestedScopes() {
					// 判断scope合法性
					if _, has := ucenter.Scopes[scope]; !has {
						oauth2provider.WriteAuthorizeError(c.Writer, ar, fosite.ErrInvalidRequest)
						break
					}
					if len(user.UserAuthorizeds) == 1 {
						checkPerms[scope] = user.UserAuthorizeds[0].Permission[scope]
					} else {
						checkPerms[scope] = true
					}
				}

				// 权限授予界面
				c.HTML(http.StatusOK, "page/auth", nbgin.Data(c, gin.H{
					"User":   user,
					"Client": ar.GetClient(),
					"Check":  checkPerms,
					"Scopes": ucenter.Scopes,
				}))
				return
			}
		} else if c.Request.Method == http.MethodPost {
			// 用户选择了授权的权限
			var perms = make(map[string]bool)
			for _, scope := range ar.GetRequestedScopes() {
				if _, has := ucenter.Scopes[scope]; !has {
					oauth2provider.WriteAuthorizeError(c.Writer, ar, err)
					return
				}
				perms[scope] = c.PostForm(scope) == "on"
			}
			if len(user.UserAuthorizeds) == 0 {
				user.UserAuthorizeds = make([]ucenter.UserAuthorized, 0)
				user.UserAuthorizeds = append(user.UserAuthorizeds, ucenter.UserAuthorized{})
			}
			user.UserAuthorizeds[0].Scope = pq.StringArray(ar.GetRequestedScopes())
			user.UserAuthorizeds[0].Permission = perms
			user.UserAuthorizeds[0].UserID = user.ID
			user.UserAuthorizeds[0].ClientID = ar.GetClient().GetID()
			// 新增授权还是更新授权
			if err := ucenter.DB.Save(&user.UserAuthorizeds[0]).Error; err != nil {
				oauth2provider.WriteAuthorizeError(c.Writer, ar, err)
				return
			}
		} else {
			oauth2provider.WriteAuthorizeError(c.Writer, ar, fosite.ErrInvalidRequest)
			return
		}
		scop := make([]byte, 0)
		for k, v := range user.UserAuthorizeds[0].Permission {
			if v {
				ar.GrantScope(k)
				scop = append(scop, []byte(k+" ")...)
			}
		}
		mySessionData := storage.NewFositeSession(user.StrID())

		response, err := oauth2provider.NewAuthorizeResponse(ctx, ar, mySessionData)

		if err != nil {
			log.Printf("Error occurred in NewAuthorizeResponse: %+v", err)
			oauth2provider.WriteAuthorizeError(c.Writer, ar, err)
			return
		}

		// Last but not least, send the response!
		oauth2provider.WriteAuthorizeResponse(c.Writer, ar, response)
	} else {
		// 用户未登录，跳转登录界面
		nbgin.SetNoCache(c)
		c.Redirect(http.StatusFound, "/login?return_url="+url.QueryEscape(c.Request.RequestURI))
	}
}

func oauth2token(c *gin.Context) {
	resp := osinServer.NewResponse()
	defer resp.Close()

	if ar := osinServer.HandleAccessRequest(resp, c.Request); ar != nil {
		switch ar.Type {
		case osin.AUTHORIZATION_CODE:
			ar.Authorized = true
		case osin.REFRESH_TOKEN:
			ar.Authorized = true
		case osin.PASSWORD:
			if ar.Username == "test" && ar.Password == "test" {
				ar.Authorized = true
			}
		case osin.CLIENT_CREDENTIALS:
			ar.Authorized = true
		case osin.ASSERTION:
			if ar.AssertionType == "urn:nb.unknown" && ar.Assertion == "very.newbie" {
				ar.Authorized = true
			}
		}
		osinServer.FinishAccessRequest(resp, c.Request, ar)

		// If an ID Token was encoded as the UserData, serialize and sign it.
		var id IDToken
		if err := json.Unmarshal([]byte(ar.UserData.(string)), &id); err == nil {
			encodeIDToken(resp, id, jwtSigner)
		}
	}
	if resp.IsError && resp.InternalError != nil {
		fmt.Printf("ERROR: %s\n", resp.InternalError)
	}
	osin.OutputJSON(resp, c.Writer, c.Request)
}
