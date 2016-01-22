package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	_ "./src/mysql"
	"./src/redigo/redis"
)

//----------------------------------
// Entity Abstracts
//----------------------------------
type User struct {
	Id          int
	UserName    string
	PassWord    string
	AccessToken string
}

type Food struct {
	Id    int `json:"id"`
	Price int `json:"price"`
	Stock int `json:"stock"`
}

//----------------------------------
// Global Variables
//----------------------------------
var (
	gUsersByName    = make(map[string]User)   // map[username]users
	gUsersById      = make(map[int]User)      // map[userId]users
	gUsersIdByToken = make(map[string]string) // map[token]userId
	gFoodsById      = make(map[int]Food)      // map[food.Id]food
	gFoodsAll       = make([]Food, 0)
)

var (
	gUserIdMapLock sync.RWMutex
	gFoodsJsonData []byte
)

var (
	gRedisPool *redis.Pool
)

const (
	const_food_info_split = ";"
	const_food_item_split = ","
	const_food_data_split = ":"

	const_token_2_userid_prefix = "t2u:"
	const_food_2_count_prefix   = "f2c:"
	const_cart_2_info_prefix    = "c2i:"
	const_all_order_prefix      = "a2o:"
	const_user_order_prefix     = "u2o:"
)

func byte2string(b []byte) string {
	for i := 0; i < len(b); i++ {
		if b[i] == 0 {
			return string(b[0:i])
		}
	}
	return string(b)
}

func string2byte(s string) []byte {
	return []byte(s)
}

func clear() bool {
	c := gRedisPool.Get()
	defer c.Close()
	b, _ := redis.Bool(c.Do("FLUSHALL"))
	return b
}

func getList(v string) []string {
	c := gRedisPool.Get()
	defer c.Close()
	reply, err := redis.Strings(c.Do("lrange", v, 0, -1))
	if err != nil {
		return nil
	}
	return reply
}

func setList(k string, v string) bool {
	c := gRedisPool.Get()
	defer c.Close()
	b, _ := redis.Bool(c.Do("lpush", k, v))
	return b
}

func get(v string) string {
	c := gRedisPool.Get()
	defer c.Close()
	s, _ := redis.String(c.Do("GET", v))
	return s
}

func set(k string, v string) bool {
	c := gRedisPool.Get()
	defer c.Close()
	b, _ := redis.Bool(c.Do("SET", k, v))
	return b
}

func mget(v string) []string {
	c := gRedisPool.Get()
	defer c.Close()
	reply, err := redis.Strings(c.Do("MGET", v))
	if err != nil {
		return nil
	}
	return reply
}

func mset(k string, v string) bool {
	c := gRedisPool.Get()
	defer c.Close()
	b, _ := redis.Bool(c.Do("MSET", k, v))
	return b
}

func incrBy(k string, v int) int {
	c := gRedisPool.Get()
	defer c.Close()
	n, _ := redis.Int(c.Do("INCRBY", k, strconv.Itoa(v)))
	return n
}

func decrBy(k string, v int) int {
	c := gRedisPool.Get()
	defer c.Close()
	n, _ := redis.Int(c.Do("DECRBY", k, strconv.Itoa(v)))
	return n
}

// 获取UserId，方法先查找内存，内存没命中，再查找redis
func getUserIdByToken(token string) string {
	// 先查内存
	// 内存没命中再查找redis
	userId := ""
	ok := false

	// 读锁
	gUserIdMapLock.RLock()
	userId, ok = gUsersIdByToken[token]
	gUserIdMapLock.RUnlock()

	if ok == false {
		userId = get(const_token_2_userid_prefix + token)
		if userId != "" {
			// 写锁
			gUserIdMapLock.Lock()
			gUsersIdByToken[token] = userId
			gUserIdMapLock.Unlock()
		}
	}

	return userId
	/*
		data := strings.Split(token, "_")
		if data != nil && data[0] == "fuck" {
			return data[1]
		}
	*/
	return ""
}

func setUserIdByToken(token string, userId string) bool {
	ok := set(const_token_2_userid_prefix+token, userId)
	if ok == true {
		// 插入内存
		gUserIdMapLock.Lock()
		gUsersIdByToken[token] = userId
		gUserIdMapLock.Unlock()
	}

	return ok
}

// 原子操作库存
func setFoodCount(foodId int, foodCount int) bool {
	return set(const_food_2_count_prefix+strconv.Itoa(foodId), strconv.Itoa(foodCount))
}

func incFoodCount(foodId int, foodCount int) int {
	return incrBy(const_food_2_count_prefix+strconv.Itoa(foodId), foodCount)
}

func decFoodCount(foodId int, foodCount int) int {
	return decrBy(const_food_2_count_prefix+strconv.Itoa(foodId), foodCount)
}

// 获取订单
func getAllOrderData() []ResponseOrderData {
	orderDatas := make([]ResponseOrderData, 0)
	datas := getList(const_all_order_prefix)
	if datas != nil {
		for i := 0; i < len(datas); i++ {
			orderData := ResponseOrderData{}
			err := json.Unmarshal(string2byte(datas[i]), &orderData)
			if err == nil {
				orderDatas = append(orderDatas, orderData)
			}
		}
	}
	return orderDatas
}

// 存入所有订单数据
func setAllOrder(order ResponseOrderData) bool {
	content, err := json.MarshalIndent(order, "", "  ")
	if err != nil {
		return false
	}
	return setList(const_all_order_prefix, byte2string(content))
}

// 获取订单数据
func getOrderData(userId string) []ResponseOrderData {
	orderDatas := make([]ResponseOrderData, 0)
	data := get(const_user_order_prefix + userId)
	if data != "" {
		orderData := ResponseOrderData{}
		err := json.Unmarshal(string2byte(data), &orderData)
		if err == nil {
			orderDatas = append(orderDatas, orderData)
		}
	}
	return orderDatas
}

// 存入订单数据
func setOrderData(userId string, order ResponseOrderData) bool {
	content, err := json.MarshalIndent(order, "", "  ")
	if err != nil {
		return false
	}
	return set(const_user_order_prefix+userId, string(content))
}

// 获取购物车信息
func getCartInfo(cartId string) (bool, string, []Food) {
	cartInfo := get(const_cart_2_info_prefix + cartId)
	if cartInfo == "" {
		return false, "", nil
	}

	info := strings.Split(cartInfo, const_food_info_split)
	if info == nil {
		return false, "", nil
	}

	userId := info[0]
	if len(info) == 1 {
		return true, userId, nil
	}

	var foodData Food
	foodsData := make([]Food, 0)
	foodsItem := strings.Split(info[1], const_food_item_split)
	for i := 0; i < len(foodsItem); i++ {
		data := strings.Split(foodsItem[i], const_food_data_split)
		if data != nil && len(data) == 2 {
			foodData.Id, _ = strconv.Atoi(data[0])
			foodData.Stock, _ = strconv.Atoi(data[1])
			foodsData = append(foodsData, foodData)
		}
	}

	return true, userId, foodsData
}

// 存入购物车信息
func setCartInfo(cartId string, userId string, foodsData []Food) bool {
	var buffer bytes.Buffer
	buffer.WriteString(userId)
	buffer.WriteString(const_food_info_split)

	if foodsData != nil {
		for i := 0; i < len(foodsData); i++ {
			buffer.WriteString(strconv.Itoa(foodsData[i].Id))
			buffer.WriteString(const_food_data_split)
			buffer.WriteString(strconv.Itoa(foodsData[i].Stock))
			buffer.WriteString(const_food_item_split)
		}
	}

	return set(const_cart_2_info_prefix+cartId, buffer.String())
}

// 读出用户信息
func loadUsers(db *sql.DB) {
	var user User
	rows, err := db.Query("SELECT `id`, `name`, `password` from user")
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		err = rows.Scan(&user.Id, &user.UserName, &user.PassWord)
		if err != nil {
			panic(err)
		}
		gUsersByName[user.UserName] = user
		gUsersById[user.Id] = user
	}
}

// 读出食品信息
func loadFoods(db *sql.DB) {
	var food Food
	rows, err := db.Query("SELECT `id`, `stock`, `price` from food")
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		err = rows.Scan(&food.Id, &food.Stock, &food.Price)
		if err != nil {
			panic(err)
		}
		gFoodsById[food.Id] = food
		gFoodsAll = append(gFoodsAll, food)

		// 写入Redis缓存
		setFoodCount(food.Id, food.Stock)
	}

	// 预先格式化为Json数据
	gFoodsJsonData, _ = json.MarshalIndent(gFoodsAll, "", "  ")
}

// 创建Redis链接池
func newPool(ip string, port string) *redis.Pool {
	return &redis.Pool{
		MaxIdle: 600,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", ip+":"+port)
			if err != nil {
				panic(err.Error())
			}
			return c, err
		},
	}
}

func initDao() {
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")

	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")

	if dbHost == "" {
		dbHost = "localhost"
	}
	if dbPort == "" {
		dbPort = "3306"
	}
	if dbName == "" {
		dbName = "eleme"
	}
	if dbUser == "" {
		dbUser = "root"
	}
	if dbPass == "" {
		dbPass = "toor"
	}
	if redisHost == "" {
		redisHost = "localhost"
	}
	if redisPort == "" {
		redisPort = "6379"
	}

	fmt.Printf("Connect to redis..")
	gRedisPool = newPool(redisHost, redisPort)
	fmt.Printf("OK\n")

	// 清空redis
	clear()

	fmt.Printf("Connect to mysql..")
	dbDsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUser, dbPass, dbHost, dbPort, dbName)
	db, err := sql.Open("mysql", dbDsn)
	if err != nil {
		panic(err)
	}

	defer db.Close()
	fmt.Printf("OK\n")

	fmt.Printf("Ping to mysql..")
	err = db.Ping()
	if err != nil {
		panic(err)
	}
	fmt.Printf("OK\n")

	fmt.Printf("Load users from mysql..")
	loadUsers(db)
	fmt.Printf("OK\n")

	fmt.Printf("Load foods from mysql..")
	loadFoods(db)
	fmt.Printf("OK\n")

	fmt.Printf("DataBase init OK!\n")
}
