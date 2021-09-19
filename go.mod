module github.com/night-codes/summer

go 1.16

require (
	github.com/gin-gonic/gin v1.7.4
	github.com/gorilla/websocket v1.4.2
	github.com/kennygrant/sanitize v1.2.4
	github.com/night-codes/conv v1.0.2
	github.com/night-codes/govalidator v1.0.4
	github.com/night-codes/mgo-ai v0.0.0-20190929120331-0ce697f507bb
	github.com/night-codes/mgo-wrapper v0.0.0-20160222150331-6f8cfc18b1c1
	github.com/urfave/cli v1.22.5
	golang.org/x/crypto v0.0.0-20210915214749-c084706c2272
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22
)

retract [v1.0.0, v1.7.0]
