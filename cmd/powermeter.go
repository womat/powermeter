package main

import (
	"fmt"
	"time"

	"powermeter/global"
	_ "powermeter/pkg/config"
	"powermeter/pkg/debug"
	"powermeter/pkg/energy"
	"powermeter/pkg/mbclient"
	"powermeter/pkg/mbgw"
)

type Energy struct {
	from  time.Time
	to    time.Time
	E     float64
	Power float64
}
type HistoryEnergy struct {
	E        []Energy
	lastE    float64
	lastTime time.Time
}

type Client struct {
	History HistoryEnergy
	Client  energy.Meter
}

func main() {
	Clients := map[string]Client{}
	for n, client := range global.Config.Clients {
		switch t := client.Type; t {
		case "mbclient":
			c := mbclient.NewClient()
			if err := c.Listen(client.Connection); err != nil {
				debug.Errorlog.Printf("error to start modbus client %v: %v\n", client.Connection, err)
				return
			}
			Clients[n] = Client{Client: c,
				History: HistoryEnergy{E: []Energy{}}}

		case "mbgateway":
			c := mbgw.NewClient()
			if err := c.Listen(client.Connection); err != nil {
				debug.Errorlog.Printf("error to start modbus gateway client %v: %v\n", client.Connection, err)
				return
			}
			Clients[n] = Client{Client: c,
				History: HistoryEnergy{E: []Energy{}}}

		case "fritz!powerline":
		default:
			debug.Warninglog.Printf("client type %v is not supported\n", t)
		}
	}

	// TODO: Garbage Collector (cleanup HistoryEnergy)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {

		for name, c := range Clients {
			t := time.Now()
			e, _ := c.Client.GetEnergyCounter()

			if c.History.lastE > 0 {
				energy := Energy{
					from:  c.History.lastTime,
					to:    t,
					E:     e,
					Power: (e - c.History.lastE) / t.Sub(c.History.lastTime).Seconds() * 60 * 60,
				}

				if name == "primarymeter" || name == "heatpump" {
					fmt.Println(name, energy.Power, (e - c.History.lastE))
				}
				c.History.E = append(c.History.E, energy)
			}

			c.History.lastE = e
			c.History.lastTime = t
			Clients[name] = c
		}

		select {
		case <-ticker.C:
		}
	}
}

/*
func (c *Client) GetLastTime() (lt time.Time) {
	for _, h := range c.History.E {
		if h.to.After(lt) {
			lt = h.to
		}
	}

	return
}

func (c *Client) GetLastECounter() (ec float64) {
	var lt time.Time
	for _, h := range c.History.E {
		if h.to.After(lt) {
			lt = h.to
			ec = h.E
		}
	}

	return
}


*/
