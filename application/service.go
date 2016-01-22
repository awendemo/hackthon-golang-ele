package main

import (
	"strconv"
	"strings"

	"./src/uuid"
)

func genUUID() string {
	return uuid.Rand().Hex()
}

func createAccessToken(userId int) string {
	accessToken := genUUID()
	setUserIdByToken(accessToken, strconv.Itoa(userId))
	return accessToken
	/*
	  TODO: 优化Token算法，和前几名差距在这里
		return "fuck_" + strconv.Itoa(userId);
	*/
}

func createCartId(userId string) string {
	cartId := genUUID()
	setCartInfo(cartId, userId, nil)
	return cartId
}

func createOrderId(userId string) string {
	oderId := genUUID()
	return oderId
}

func addFood2CartId(cartId string, userId string, foodId int, foodCount int) int {
	b, retUserId, foodsData := getCartInfo(cartId)
	// 篮子不存在
	if b == false {
		return -3
	}

	// 没权限
	if strings.EqualFold(retUserId, userId) == false {
		return -4
	}

	// 食物不存在
	food, ok := gFoodsById[foodId]
	if ok == false {
		return -1
	}

	add := true
	count := 0

	for i := 0; i < len(foodsData); i++ {
		if foodsData[i].Id == foodId {
			foodsData[i].Stock += foodCount
			add = false
		}
		count += foodsData[i].Stock
	}

	if add == true {
		food = Food{}
		food.Id = foodId
		food.Stock = foodCount
		foodsData = append(foodsData, food)
		count += foodCount
	}

	// 超过数量
	if count > 3 {
		return -2
	}

	setCartInfo(cartId, userId, foodsData)

	return 0
}

func makeOrder(cartId string, userId string) (int, string) {
	// 查询Redis
	oderDatas := getOrderData(userId)

	// 不能再次下单
	if oderDatas != nil && len(oderDatas) > 0 {
		return -3, ""
	}

	b, retUserId, retFoodsData := getCartInfo(cartId)
	// 篮子不存在
	if b == false {
		return -1, ""
	}

	// 没权限
	if strings.EqualFold(retUserId, userId) == false {
		return -2, ""
	}

	oderData := ResponseOrderData{}
	oderData.UserId, _ = strconv.Atoi(userId)
	oderData.Total = 0

	for i := 0; i < len(retFoodsData); i++ {
		remain := decFoodCount(retFoodsData[i].Id, retFoodsData[i].Stock)
		// 库存不足
		if remain < 0 {
			incFoodCount(retFoodsData[i].Id, retFoodsData[i].Stock)
			return -4, ""
		}
		oderData.Items = append(oderData.Items, ResponseOrderItem{retFoodsData[i].Id, retFoodsData[i].Stock})
		oderData.Total += gFoodsById[retFoodsData[i].Id].Price
	}

  // TODO: 写入redis归纳为一次，和前几名差距在这里，采用直接拆分凡是
	oderData.Id = createOrderId(userId)

	// 购买信息，并写入Redis1
	setOrderData(userId, oderData)
	setAllOrder(oderData)

	return 0, oderData.Id
}

func getOrder(userId string) (int, []ResponseOrderData) {
	ret := 0
	return ret, getOrderData(userId)
}

func getAllOrder(userId string) (int, []ResponseOrderData) {
	id, _ := strconv.Atoi(userId)
	ret := 0
	if user, ok := gUsersById[id]; ok {
		if user.UserName != "root" {
			ret = -1
		}
	}
	return ret, getAllOrderData()
}
