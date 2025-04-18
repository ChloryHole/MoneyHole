package aggregator

import (
	"context"
	//"fmt"
	"log"
	"os/signal"
	"syscall"
	"time"
	"database/sql"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/russianinvestments/invest-api-go-sdk/investgo"
	_ "github.com/russianinvestments/invest-api-go-sdk/proto"
)

func Run() {
	config, err := investgo.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("config loading error %v", err.Error())
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	defer cancel()
	// сдк использует для внутреннего логирования investgo.Logger
	// для примера передадим uber.zap
	zapConfig := zap.NewDevelopmentConfig()
	zapConfig.EncoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout(time.DateTime)
	zapConfig.EncoderConfig.TimeKey = "time"
	l, err := zapConfig.Build()
	logger := l.Sugar()
	defer func() {
		err := logger.Sync()
		if err != nil {
			log.Print(err.Error())
		}
	}()
	if err != nil {
		log.Fatalf("logger creating error %v", err)
	}
	// создаем клиента для investAPI, он позволяет создавать нужные сервисы и уже
	// через них вызывать нужные методы

	// если вы хотите передать опции для создания соединения grpc.ClientConnOption, то передайте их в NewClient
	// client, err := investgo.NewClient(ctx, config, logger, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1e6)))
	client, err := investgo.NewClient(ctx, config, logger)
	if err != nil {
		logger.Fatalf("client creating error %v", err.Error())
	}
	defer func() {
		logger.Infof("closing client connection")
		err := client.Stop()
		if err != nil {
			logger.Errorf("client shutdown error %v", err.Error())
		}
	}()

	db, err := sql.Open("sqlite3", "my.sql")
	if err != nil {
		log.Print(err.Error())
		return
	}
	defer db.Close()

	rows, err := db.Query(`SELECT id FROM quotes`)
	if err != nil {
		log.Print(err.Error())
		return 
	}
	defer rows.Close()

	instruments := make([]string, 0)
	for rows.Next() {
		var s string
		err = rows.Scan(&s,) 
		if err != nil {
			log.Print(err.Error())
            return 
        }
		instruments = append(instruments, s)
	}

	// создаем клиента для сервиса маркетдаты
	MarketDataService := client.NewMarketDataServiceClient()
	//instruments := []string{"BBG004730N88", "BBG00475KKY8", "BBG004RVFCY3"}

	lastPriceResp, err := MarketDataService.GetLastPrices(instruments)
	if err != nil {
		logger.Error(err.Error())
		return 
	} 
	lp := lastPriceResp.GetLastPrices()
	/*for i, price := range lp {
		fmt.Printf("last price number %v = %v\n", i, price.GetPrice().ToFloat())
	}*/
	for i, v := range instruments {
		_, err = db.Exec(`Update quotes SET cost = $1 WHERE id = $2`, 
			lp[i].GetPrice().ToFloat(),
			v,
		)
		if err != nil {
			log.Print(err.Error())
			return
		}
	} 
}