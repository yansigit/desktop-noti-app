package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"github.com/kataras/iris/v12"
	"github.com/kataras/iris/v12/context"
	"kiosk-uou-pos/data"
	"log"
	"net"
	"strconv"
	"time"
)

func main() {
	fmt.Println("※ 키오스크 서버입니다. 창을 닫지 말아주세요. ※")
	fmt.Println("※ 웹 페이지의 알람 초기화 버튼을 눌러 알람기능을 동작시켜주세요. ※")
	initWeb()
}

func initWeb() {
	app := iris.New()

	tmpl := iris.Django("./templates", ".html")
	tmpl.Reload(true)                             // reload templates on each request (development mode)
	tmpl.AddFunc("greet", func(s string) string { // {{greet(name)}}
		return "Greetings " + s + "!"
	})

	// tmpl.RegisterFilter("myFilter", myFilter) // {{"simple input for filter"|myFilter}}
	app.RegisterView(tmpl)
	app.HandleDir("/assets", "./assets")

	app.Get("/", index)
	// 새 주문 들어왔느지 확인
	// app.Get("/queue", queue)
	// 왼쪽 오더 리스트
	app.Get("/orders", refreshedOrderList)
	app.Get("/orders/{page:int}", refreshedOrderList)
	// 오른쪽 대기번호 조정 패널
	app.Get("/waits", refreshWaitNumbers)
	// 외부 모니터 대기번호 출력
	app.Get("/out-display", outDisplay)
	// 정산
	app.Get("/jungsan", jungSan)
	app.Get("/jungsan/{date:string}", jungSan)

	app.Post("/", storeOrderList)
	app.Post("/action", action)

	app.Connect("/", func(context *context.Context) {
		_, _ = context.ResponseWriter().Write([]byte("Hello World"))
	})

	// http://localhost:8080
	_ = app.Listen(":8080")
}

func index(ctx iris.Context) {
	_ = ctx.View("index.html")
}

func outDisplay(ctx iris.Context) {
	var confirmedOrders []data.Order
	var unconfirmedOrders []data.Order

	confirmedOrders = data.FindOrderListWithStatus(1, 5)
	unconfirmedOrders = data.FindOrderListWithStatus(0, 10)

	var unconfirmedOrdersArray [][]data.Order
	if len(unconfirmedOrders) > 5 {
		unconfirmedOrdersArray = append(unconfirmedOrdersArray, unconfirmedOrders[:5], unconfirmedOrders[5:])
	} else {
		unconfirmedOrdersArray = append(unconfirmedOrdersArray, unconfirmedOrders)
	}

	_ = ctx.View("displayWaitingNumbers.html", iris.Map{
		"confirmed_orders":         confirmedOrders,
		"unconfirmed_orders_array": unconfirmedOrdersArray,
	})
}

func refreshedOrderList(ctx iris.Context) {
	var orders []data.Order
	page, err := ctx.Params().GetInt("page")
	if err != nil {
		// fmt.Print(err)
		page = 1
	}
	data.Paging(page, &orders)

	for i, _ := range orders {
		var menus []data.Menu
		data.GetMenusFromOrder(orders[i], &menus)
		orders[i].Menus = make([]data.Menu, len(menus))
		copy(orders[i].Menus, menus)

		for j, _ := range orders[i].Menus {
			var options []data.Option
			data.GetOptionsFromMenu(orders[i].Menus[j], &options)
			orders[i].Menus[j].Options = make([]data.Option, len(options))
			copy(orders[i].Menus[j].Options, options)
		}
	}

	args := map[string]interface{}{
		"order_list": orders,
	}

	buf := new(bytes.Buffer)
	ctx.Application().View(buf, "refreshedOrderList.html", "refreshedOrderList.html", args)
	ctx.WriteString(buf.String())
}

func refreshWaitNumbers(ctx iris.Context) {
	var confirmedOrders []data.Order
	var unconfirmedOrders []data.Order

	confirmedOrders = data.FindOrderListWithStatus(1, 40)
	unconfirmedOrders = data.FindOrderListWithStatus(0, 40)

	var confirmedOrdersMashed [][]data.Order
	length := len(confirmedOrders)
	for i := 0; i < length; i += 3 { // 0,1,2,3,4,5 length:6
		if i >= length {
			break
		}
		if length-i < 3 {
			if length-i == 1 {
				confirmedOrdersMashed = append(confirmedOrdersMashed, []data.Order{confirmedOrders[i]})
			} else if length-i == 2 {
				confirmedOrdersMashed = append(confirmedOrdersMashed, []data.Order{confirmedOrders[i], confirmedOrders[i+1]})
			}
			break
		}
		confirmedOrdersMashed = append(confirmedOrdersMashed, []data.Order{confirmedOrders[i], confirmedOrders[i+1], confirmedOrders[i+2]})
	}

	var unconfirmedOrdersMashed [][]data.Order
	length = len(unconfirmedOrders)
	for i := 0; i < length; i += 3 { // 0,1,2,3,4,5 length:6
		if i >= length {
			break
		}
		if length-i < 3 {
			if length-i == 1 {
				unconfirmedOrdersMashed = append(unconfirmedOrdersMashed, []data.Order{unconfirmedOrders[i]})
			} else if length-i == 2 {
				unconfirmedOrdersMashed = append(unconfirmedOrdersMashed, []data.Order{unconfirmedOrders[i], unconfirmedOrders[i+1]})
			}
			break
		}
		unconfirmedOrdersMashed = append(unconfirmedOrdersMashed, []data.Order{unconfirmedOrders[i], unconfirmedOrders[i+1], unconfirmedOrders[i+2]})
	}

	args := map[string]interface{}{
		"unconfirmed_orders": unconfirmedOrdersMashed,
		"confirmed_orders":   confirmedOrdersMashed,
	}

	buf := new(bytes.Buffer)
	ctx.Application().View(buf, "waitingNumberTable.html", "refreshedOrderList.html", args)
	ctx.WriteString(buf.String())
}

func action(ctx iris.Context) {
	action := ctx.PostValue("action")
	if action == "confirm" {
		orderNumber, _ := strconv.Atoi(ctx.PostValue("orderNumber"))
		data.UpdateOrderListConfirmation(uint(orderNumber))
		_, _ = ctx.Text("confirmed")
		return
	} else if action == "cancel" {
		orderNumber, _ := strconv.Atoi(ctx.PostValue("orderNumber"))
		data.CancelOrderList(uint(orderNumber))
		_, _ = ctx.Text("canceled")
		return
	} else if action == "insertbogus" {
		data.InsertBogusOrderList()
		_, _ = ctx.Text("inserted bogus")
		return
	} else if action == "printJungsan" {
		result := printJungsan(ctx.PostValue("body"))
		if result {
			ctx.Write([]byte("ok"))
		} else {
			ctx.Write([]byte("fail"))
		}
		return
	} else if action == "reprint" {
		orderNumber, _ := strconv.Atoi(ctx.PostValue("orderNumber"))
		order := data.FindOrderList(uint(orderNumber))

		var menus []data.Menu
		data.GetMenusFromOrder(order, &menus)
		order.Menus = menus

		for j, _ := range order.Menus {
			var options []data.Option
			data.GetOptionsFromMenu(order.Menus[j], &options)
			order.Menus[j].Options = make([]data.Option, len(options))
			copy(order.Menus[j].Options, options)
		}

		var m map[string]interface{}
		jsonBytes, err := json.Marshal(order)
		if err != nil {
			panic("JSON으로 구조체를 변경하는데 문제가 있습니다")
		}
		err = json.Unmarshal(jsonBytes, &m)
		if err != nil {
			panic("JSON을 맵으로 변경하는데 문제가 있습니다")
		}
		m["action"] = "reprint"
		// m["orderNum"] = m["ID"]
		m["orderNum"] = m["TodayIndex"]
		jsonBytes, err = json.Marshal(m)
		if err != nil {
			panic("맵을 JSON으로 변경하는데 문제가 있습니다")
		}
		printWithThermalPrinter(jsonBytes)
		_, _ = ctx.Text("reprinted")
		return
	}
	_, _ = ctx.Text("알 수 없는 명령입니다")
	return
}

var newOrderAvailable bool = false

func printJungsan(data string) bool {
	jungsanData := []byte(data)
	fmt.Println(data)

	connection, err := net.Dial("tcp", ":13522")
	if err != nil {
		log.Printf("프린터 서버가 켜져있지 않습니다.")
		return false
	}

	_, err = connection.Write(jungsanData)
	if err != nil {
		log.Printf("정산 내용이 들어왔습니다. 프린트를 시작합니다.")
	}
	err = connection.Close()
	return true
}

func storeOrderList(ctx iris.Context) {
	order, _ := ctx.GetBody()
	fmt.Printf("%x\n", md5.Sum(order))
	result := data.InsertOrderList(order)

	newOrderAvailable = true

	response := iris.Map{"state": "OK", "orderNumber": result.TodayIndex}
	ctx.JSON(response)
	println(response)

	var m map[string]interface{}
	err := json.Unmarshal(order, &m)
	m["orderNum"] = result.TodayIndex
	newOrderData, err := json.Marshal(m)
	if err != nil {
		log.Println("JSON 주문번호 추가 과정에서 에러가 발생했습니다")
	}
	printWithThermalPrinter(newOrderData)
}

func printWithThermalPrinter(jsonBytes []byte) {
	connection, err := net.Dial("tcp", ":13522")
	if err != nil {
		fmt.Println(err)
		log.Printf("프린터 서버가 켜져있지 않습니다.")
	} else {
		_, err := connection.Write(jsonBytes)
		if err != nil {
			log.Printf("주문 프린트 요청이 입력되었습니다. 프린트를 시작합니다.")
		}
		err = connection.Close()
	}
}

func queue(ctx iris.Context) {
	ctx.JSON(iris.Map{"new": newOrderAvailable})
	newOrderAvailable = false
}

func jungSan(ctx iris.Context) {
	var orders []data.Order
	tmpDate := ctx.Params().GetString("date")

	var date time.Time
	var err error
	isMonth := false

	if tmpDate == "" {
		date = time.Now()
	} else if len(tmpDate) == 8 {
		date, err = time.Parse("20060102", ctx.Params().Get("date"))
		if err != nil {
			date = time.Now()
		}
	} else if len(tmpDate) == 6 {
		isMonth = true
		date, err = time.Parse("200601", ctx.Params().Get("date"))
		if err != nil {
			date = time.Now()
		}
	} else {
		_, _ = ctx.Text("날짜 형식에 문제가 있습니다")
		return
	}
	data.ChangeDBFile(date.Format("200601"))

	if isMonth {
		orders = data.FindOrderListWithMonth(date)
	} else {
		orders = data.FindOrderListWithDate(date)
	}

	totalPrice := 0
	canceledCnt, canceledPrice := 0, 0
	discountCnt, discountPrice := 0, 0

	// 메뉴별 수량, 금액 카운트용
	menuTable := make(map[string][]int)

	for i := 0; i < len(orders); i++ {
		data.Db.Model(&orders[i]).Association("Menus").Find(&orders[i].Menus)
		if orders[i].IsConfirmed == 3 {
			canceledCnt, canceledPrice = canceledCnt+1, canceledPrice+orders[i].TotalPrice
		}
		for _, menu := range orders[i].Menus {
			if menu.IsTumbler {
				discountCnt, discountPrice = discountCnt+1, discountPrice+200
			}
			if menuTable[menu.Name] == nil {
				menuTable[menu.Name] = []int{0, 0}
			}
			menuTable[menu.Name][0] += 1
			menuTable[menu.Name][1] += menu.TotalPrice
		}
		totalPrice += orders[i].TotalPrice
	}

	args := map[string]interface{}{
		"switch":        "일별",
		"order_list":    orders,
		"date":          date.Format("2006-01-02"),
		"canceledCnt":   canceledCnt,
		"canceledPrice": canceledPrice,
		"discountCnt":   discountCnt,
		"discountPrice": discountPrice,
		"totalPrice":    totalPrice,
		"menuTable":     menuTable,
	}

	if isMonth {
		args["date"] = date.Format("2006-01")
		args["switch"] = "월별"
	}

	data.ChangeDBFile(time.Now().Format("200601"))

	_ = ctx.View("jungsan.html", args)
}
