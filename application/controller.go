package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	_ "strings"

	"./src/routes"
)

const (
	const_URL_AccessToken    = "access_token"
	const_Header_AccessToken = "Access-Token"
)

//----------------------------------
// Request JSON Bindings
//----------------------------------
type RequestLogin struct {
	UserName string `json:"username"`
	PassWord string `json:"password"`
}

type RequestCartAddFood struct {
	FoodId int `json:"food_id"`
	Count  int `json:"count"`
}

type RequestMakeOrder struct {
	CartId string `json:"cart_id"`
}

//----------------------------------
// Response JSON Bindings
//----------------------------------
type ResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ResponseLogin struct {
	UserId      int    `json:"user_id"`
	UserName    string `json:"username"`
	AccessToken string `json:"access_token"`
}

type ResponseGetFoods []Food

type ResponseCreateCart struct {
	CartId string `json:"cart_id"`
}

type ResponseMakeOrder struct {
	Id string `json:"id"`
}

type ResponseOrderItem struct {
	FoodId int `json:"food_id"`
	Count  int `json:"count"`
}

type ResponseOrderData struct {
	UserId int                 `json:"user_id"`
	Id     string              `json:"id"`
	Items  []ResponseOrderItem `json:"items"`
	Total  int                 `json:"total"`
}

type ResponseGetOrderData []ResponseOrderData

//
// Http 编解码
//
func httpDecode(r *http.Request, v interface{}) int {
	err := routes.ReadJson(r, v)
	if err != nil {
		fmt.Println(err)
		return -1
	}

	return 0
}

func httpEncode(w http.ResponseWriter, v interface{}, code int) {
	routes.WriteJson(w, v, code)
}

func httpEncodeContent(w http.ResponseWriter, v []byte, code int) {
	routes.WriteContent(w, v, code)
}

func httpEncodeText(w http.ResponseWriter, v string, code int) {
	routes.WriteText(w, v, code)
}

//
// 登陆 POST /login
//
func loginPost(w http.ResponseWriter, r *http.Request) {
	responseError := ResponseError{}
	if r.ContentLength == 0 {
		responseError.Code = "EMPTY_REQUEST"
		responseError.Message = "请求体为空"
		httpEncode(w, responseError, 400)
		return
	}

	requestLogin := RequestLogin{}
	ret := httpDecode(r, &requestLogin)
	if ret == -1 {
		responseError.Code = "MALFORMED_JSON"
		responseError.Message = "格式错误"
		httpEncode(w, responseError, 400)
		return
	}

	if user, ok := gUsersByName[requestLogin.UserName]; ok {
		if user.PassWord == requestLogin.PassWord {
			responseLogin := ResponseLogin{}
			responseLogin.UserId = user.Id
			responseLogin.UserName = requestLogin.UserName
			responseLogin.AccessToken = createAccessToken(user.Id)
			httpEncode(w, responseLogin, 200)
			return
		}
	}

	responseError.Code = "USER_AUTH_FAIL"
	responseError.Message = "用户名或密码错误"
	httpEncode(w, responseError, 403)
}

//
// 登陆状态判断
//
func loginStatus(w http.ResponseWriter, r *http.Request) (bool, string) {
	access_token := ""
	params := r.URL.Query()
	if params != nil {
		access_token = params.Get(const_URL_AccessToken)
	}

	if access_token == "" {
		access_token = r.Header.Get(const_Header_AccessToken)
	}

	// TODO: 优化Token算法，和前几名差距在这里，采用直接拆分凡是
	// 查找用户
	userId := getUserIdByToken(access_token)
	if userId == "" {
		responseError := ResponseError{}
		responseError.Code = "INVALID_ACCESS_TOKEN"
		responseError.Message = "无效的令牌"
		httpEncode(w, responseError, 401)

		return false, userId
	}

	return true, userId
}

//
// 查询库存 GET /foods
//
func foodsGet(w http.ResponseWriter, r *http.Request) {
	ok, _ := loginStatus(w, r)
	if ok != true {
		return
	}

	httpEncodeContent(w, gFoodsJsonData, 200)
}

//
// 创建篮子 POST /carts
//
func cartsPost(w http.ResponseWriter, r *http.Request) {
	ok, userId := loginStatus(w, r)
	if ok != true {
		return
	}

	cartId := createCartId(userId)
	requestMakeOrder := RequestMakeOrder{cartId}
	httpEncode(w, requestMakeOrder, 200)
}

//
// 添加 PATCH /carts/:cart_id
//
func cartsPatch(w http.ResponseWriter, r *http.Request) {
	ok, userId := loginStatus(w, r)
	if ok != true {
		return
	}

	responseError := ResponseError{}
	if r.ContentLength == 0 {
		responseError.Code = "EMPTY_REQUEST"
		responseError.Message = "请求体为空"
		httpEncode(w, responseError, 400)
		return
	}

	requestCartAddFood := RequestCartAddFood{}
	ret := httpDecode(r, &requestCartAddFood)
	if ret == -1 {
		responseError.Code = "MALFORMED_JSON"
		responseError.Message = "格式错误"
		httpEncode(w, responseError, 400)
		return
	}

	params := r.URL.Query()
	cartId := params.Get(":cart_id")
	// 加入购物车
	ret = addFood2CartId(cartId, userId, requestCartAddFood.FoodId, requestCartAddFood.Count)
	if ret == -1 {
		responseError.Code = "FOOD_NOT_FOUND"
		responseError.Message = "食物不存在"
		httpEncode(w, responseError, 404)
		return
	} else if ret == -2 {
		responseError.Code = "FOOD_OUT_OF_LIMIT"
		responseError.Message = "篮子中食物数量超过了三个"
		httpEncode(w, responseError, 403)
		return
	} else if ret == -3 {
		responseError.Code = "CART_NOT_FOUND"
		responseError.Message = "篮子不存在"
		httpEncode(w, responseError, 404)
		return
	} else if ret == -4 {
		responseError.Code = "NOT_AUTHORIZED_TO_ACCESS_CART"
		responseError.Message = "无权限访问指定的篮子"
		httpEncode(w, responseError, 401)
		return
	}

	httpEncode(w, nil, 204)
}

//
// 下单 POST /orders
//
func ordersPost(w http.ResponseWriter, r *http.Request) {
	ok, userId := loginStatus(w, r)
	if ok != true {
		return
	}

	responseError := ResponseError{}
	if r.ContentLength == 0 {
		responseError.Code = "EMPTY_REQUEST"
		responseError.Message = "请求体为空"
		httpEncode(w, responseError, 400)
		return
	}

	requestMakeOrder := RequestMakeOrder{}
	ret := httpDecode(r, &requestMakeOrder)
	if ret == -1 {
		responseError.Code = "MALFORMED_JSON"
		responseError.Message = "格式错误"
		httpEncode(w, responseError, 400)
		return
	}

	ret, oderId := makeOrder(requestMakeOrder.CartId, userId)
	if ret == -1 {
		responseError.Code = "CART_NOT_FOUND"
		responseError.Message = "篮子不存在"
		httpEncode(w, responseError, 404)
		return
	} else if ret == -2 {
		responseError.Code = "NOT_AUTHORIZED_TO_ACCESS_CART"
		responseError.Message = "无权限访问指定的篮子"
		httpEncode(w, responseError, 401)
		return
	} else if ret == -3 {
		responseError.Code = "ORDER_OUT_OF_LIMIT"
		responseError.Message = "每个用户只能下一单"
		httpEncode(w, responseError, 403)
		return
	} else if ret == -4 {
		responseError.Code = "FOOD_OUT_OF_STOCK"
		responseError.Message = "食物库存不足"
		httpEncode(w, responseError, 403)
		return
	}

	responseMakeOrder := ResponseMakeOrder{oderId}
	httpEncode(w, responseMakeOrder, 200)
}

//
// 查询订单 GET /orders
//
func ordersGet(w http.ResponseWriter, r *http.Request) {
	ok, userId := loginStatus(w, r)
	if ok != true {
		return
	}

	_, responseGetOrder := getOrder(userId)
	httpEncode(w, responseGetOrder, 200)
}

//
// 后台接口－查询订单 GET /admin/orders
//
func adminOrdersGet(w http.ResponseWriter, r *http.Request) {
	ok, userId := loginStatus(w, r)
	if ok != true {
		return
	}

	ret, responseGetOrder := getAllOrder(userId)
	// 不是管理员
	if ret == -1 {
		httpEncode(w, responseGetOrder, 401)
		return
	}

	httpEncode(w, responseGetOrder, 200)
}

func initController() {
	appHost := os.Getenv("APP_HOST")
	appPort := os.Getenv("APP_PORT")
	if appHost == "" {
		appHost = "localhost"
	}
	if appPort == "" {
		appPort = "8080"
	}

	urlMux := routes.New()

	urlMux.Post("/login", loginPost)
	urlMux.Get("/foods", foodsGet)
	urlMux.Post("/carts", cartsPost)
	urlMux.Patch("/carts/:cart_id", cartsPatch)
	urlMux.Post("/orders", ordersPost)
	urlMux.Get("/orders", ordersGet)
	urlMux.Get("/admin/orders", adminOrdersGet)

	http.Handle("/", urlMux)

	fmt.Printf("Start to listen " + appHost + ":" + appPort + " ...\n")

	err := http.ListenAndServe(appHost+":"+appPort, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
