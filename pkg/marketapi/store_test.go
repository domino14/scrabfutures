package marketapi

import (
	"context"
	"database/sql"
	"io/ioutil"
	"os"
	"sync"
	"testing"

	pb "github.com/domino14/scrabfutures/rpc/proto"
	"github.com/matryer/is"
)

var cfg = Config{
	DBMigrationsPath: os.Getenv("DB_MIGRATIONS_PATH"),
	DBPath:           os.Getenv("TEST_DB_PATH"),
}

func initDB() {
	os.Remove(cfg.DBPath)
	EnsureMigrations(&cfg)
}

func addFixtures(fixtureFile string) {
	bts, err := ioutil.ReadFile(fixtureFile)
	if err != nil {
		panic(err)
	}
	db, err := sql.Open("sqlite3", cfg.DBPath)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	db.BeginTx(ctx, nil)

	_, err = db.ExecContext(ctx, string(bts))
	if err != nil {
		panic(err)
	}
}

func TestFulfillOrder(t *testing.T) {
	initDB()
	addFixtures("./testfixtures/basic.sql")
	is := is.New(t)
	ctx := context.Background()
	s, err := NewSqliteStore(cfg.DBPath)
	is.NoErr(err)
	err = s.OpenMarket(ctx, "nationals2022")
	is.NoErr(err)
	err = s.FulfillOrder(ctx, "cesar", "S3uuid", "nationals2022", 50, true)
	is.NoErr(err)
	sec, err := s.GetSecurity(ctx, "S3uuid")
	is.NoErr(err)
	is.Equal(sec, &pb.Security{
		Id: "S3uuid", Description: "César wins nationals",
		Shortname: "CSAR", DateCreated: "2022-07-08T14:00:01Z",
		MarketId: "nationals2022", SharesOutstanding: 50,
		LastPrice: 35.46612443924434,
	})
}

func TestFulfillOrderNotEnoughForSale(t *testing.T) {
	initDB()
	addFixtures("./testfixtures/basic.sql")
	is := is.New(t)
	ctx := context.Background()
	s, _ := NewSqliteStore(cfg.DBPath)
	s.OpenMarket(ctx, "nationals2022")
	err := s.FulfillOrder(ctx, "cesar", "S3uuid", "nationals2022", 50, true)
	is.NoErr(err)
	// try to sell 60 shares that we don't have (we just bought 50)
	err = s.FulfillOrder(ctx, "cesar", "S3uuid", "nationals2022", 60, false)
	is.Equal(err.Error(), "cannot sell more securities than we own")
}

func TestFulfillOrderTooExpensive(t *testing.T) {
	initDB()
	addFixtures("./testfixtures/basic.sql")
	is := is.New(t)
	ctx := context.Background()
	s, _ := NewSqliteStore(cfg.DBPath)
	s.OpenMarket(ctx, "nationals2022")
	err := s.FulfillOrder(ctx, "cesar", "S3uuid", "nationals2022", 100, true)
	is.Equal(err.Error(), "not enough tokens for this transaction")
}

func TestFulfillSimultaneousOrders(t *testing.T) {
	initDB()
	addFixtures("./testfixtures/basic.sql")
	is := is.New(t)
	ctx := context.Background()
	s, _ := NewSqliteStore(cfg.DBPath)
	s.OpenMarket(ctx, "nationals2022")
	var wg sync.WaitGroup

	// Order one item simultaneously from 50 different threads. The lock should do
	// the right thing.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := s.FulfillOrder(ctx, "cesar", "S3uuid", "nationals2022", 1, true)
			is.NoErr(err)
		}()
	}
	wg.Wait()
	sec, err := s.GetSecurity(ctx, "S3uuid")
	is.NoErr(err)
	is.Equal(sec, &pb.Security{
		Id: "S3uuid", Description: "César wins nationals",
		Shortname: "CSAR", DateCreated: "2022-07-08T14:00:01Z",
		MarketId: "nationals2022", SharesOutstanding: 50,
		LastPrice: 35.46612443924434,
	})
}

func TestCreateMarket(t *testing.T) {
	initDB()
	is := is.New(t)
	ctx := context.Background()
	s, _ := NewSqliteStore(cfg.DBPath)
	uuid, err := s.CreateMarket(ctx, "a foo market")
	is.NoErr(err)
	markets, err := s.GetOpenMarkets(ctx)
	is.NoErr(err)
	is.Equal(len(markets), 0)
	err = s.OpenMarket(ctx, uuid)
	is.NoErr(err)

	markets, err = s.GetOpenMarkets(ctx)
	is.NoErr(err)
	is.Equal(len(markets), 1)
	is.Equal(markets[0], &pb.Market{
		Id:          uuid,
		Description: "a foo market",
		IsOpen:      true,
		DateCreated: markets[0].DateCreated,
	})
}

func TestDeleteMarketDisallowed(t *testing.T) {
	initDB()
	addFixtures("./testfixtures/basic.sql")
	is := is.New(t)
	ctx := context.Background()
	s, _ := NewSqliteStore(cfg.DBPath)
	err := s.OpenMarket(ctx, "nationals2022")
	is.NoErr(err)
	err = s.DeleteMarket(ctx, "nationals2022")
	is.Equal(err.Error(), "disallowed deletion of market that was once open")
}

func TestDeleteMarket(t *testing.T) {
	initDB()
	addFixtures("./testfixtures/basic.sql")
	is := is.New(t)
	ctx := context.Background()
	s, _ := NewSqliteStore(cfg.DBPath)
	err := s.DeleteMarket(ctx, "nationals2022")
	is.NoErr(err)
	// all securities for this market should be deleted by cascade.
	secs, err := s.GetSecurities(ctx, "nationals2022")
	is.NoErr(err)
	is.Equal(len(secs), 0)
}

func TestAddSecurities(t *testing.T) {
	initDB()
	is := is.New(t)
	ctx := context.Background()
	s, _ := NewSqliteStore(cfg.DBPath)
	uuid, _ := s.CreateMarket(ctx, "a foo market")

	err := s.AddSecurities(ctx, uuid, []*pb.AddSecuritiesRequest_Security{
		{Description: "someone wins nationals", Shortname: "SOMEONE"},
		{Description: "there is an alien invasion", Shortname: "ALIEN"},
	})
	is.NoErr(err)
	// check that both securities are worth 50
	secs, err := s.GetSecurities(ctx, uuid)
	is.NoErr(err)
	is.Equal(len(secs), 2)
	is.Equal(secs[0].LastPrice, 50.0)
	is.Equal(secs[1].LastPrice, 50.0)
}

func TestDeleteSecurity(t *testing.T) {
	initDB()
	addFixtures("./testfixtures/basic.sql")
	is := is.New(t)
	ctx := context.Background()
	s, _ := NewSqliteStore(cfg.DBPath)
	err := s.DeleteSecurity(ctx, "nationals2022", "S2uuid")
	is.NoErr(err)

	secs, err := s.GetSecurities(ctx, "nationals2022")
	is.NoErr(err)
	// There's only 3 securities in this market now, and the prices
	// rebalanced.
	is.Equal(len(secs), 3)
	is.Equal(secs[0].LastPrice, 100.0/3)
	is.Equal(secs[1].LastPrice, 100.0/3)
	is.Equal(secs[2].LastPrice, 100.0/3)
}
