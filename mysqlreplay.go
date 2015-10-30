package main

import (
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"os"
	"strconv"
	"time"
	"math"
)

type ReplayStatement struct {
	session int
	epoch   float64
	stmt    string
}

func timefromfloat(epoch float64) (time.Time) {
	epoch_base := math.Floor(epoch)
	epoch_frac := epoch - epoch_base
	epoch_time := time.Unix(int64(epoch_base),int64(epoch_frac*1000000000))
	return epoch_time
}

func mysqlsession(c <-chan ReplayStatement, session int, firstepoch float64, starttime time.Time) {
	fmt.Printf("NEW SESSION (session: %d)\n", session)

	db, err := sql.Open("mysql", "msandbox:msandbox@tcp(127.0.0.1:5709)/test")
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	last_stmt_epoch := firstepoch
	for {
		pkt := <-c
		if last_stmt_epoch != 0.0 {
			firsttime := timefromfloat(firstepoch)
			pkttime := timefromfloat(pkt.epoch)
			delaytime_orig := pkttime.Sub(firsttime)
			mydelay := time.Since(starttime)
			delaytime_new := delaytime_orig - mydelay

			fmt.Printf("[session %d] Sleeptime: %s\n", session, delaytime_new)
			time.Sleep(delaytime_new)
		}
		last_stmt_epoch = pkt.epoch
		fmt.Printf("[session %d] STATEMENT REPLAY: %s\n", session, pkt.stmt)
		_, err := db.Exec(pkt.stmt)
		if err != nil {
			panic(err.Error())
		}
	}
}

func main() {
	fileflag := flag.String("f", "./test.dat", "Path to datafile for replay")
	flag.Parse()

	datFile, err := os.Open(*fileflag)
	if err != nil {
		fmt.Println(err)
	}

	reader := csv.NewReader(datFile)
	reader.Comma = '\t'

	pktData, err := reader.ReadAll()
	if err != nil {
		fmt.Println(err)
	}

	var firstepoch float64 = 0.0
	starttime := time.Now()
	sessions := make(map[int]chan ReplayStatement)
	for _, stmt := range pktData {
		sessionid, err := strconv.Atoi(stmt[0])
		if err != nil {
			fmt.Println(err)
		}
		epoch, err := strconv.ParseFloat(stmt[1], 64)
		if err != nil {
			fmt.Println(err)
		}
		pkt := ReplayStatement{session: sessionid, epoch: epoch, stmt: stmt[2]}
		if firstepoch == 0.0 {
			firstepoch = pkt.epoch
		}
		if sessions[pkt.session] != nil {
			sessions[pkt.session] <- pkt
		} else {
			sess := make(chan ReplayStatement)
			sessions[pkt.session] = sess
			go mysqlsession(sessions[pkt.session], pkt.session, firstepoch, starttime)
			sessions[pkt.session] <- pkt
		}
	}
}
