package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

type weatherData struct {
	Name string `json:"name"`
	Main struct {
		Kelvin float64 `json:"temp"`
	} `json:"main"`
}

type weatherProvider interface {
	temperature(city string) (float64, error)
}

type openWeatherMap struct {}

type weatherUnderground struct {
	apiKey string
}

type multiWheaterProvider []weatherProvider

func main() {
	mw := multiWheaterProvider{
		openWeatherMap{},
		weatherUnderground{apiKey: "api-key"},
	}

	http.HandleFunc("/weather/", func(w http.ResponseWriter, r* http.Request) {
		begin := time.Now()
		city := strings.SplitN(r.URL.Path, "/", 3)[2]

		temp, err := mw.temperature(city)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json;charset=utf-8")
		json.NewEncoder(w).Encode(map[string]interface{} {
			"city": city,
			"temp": temp,
			"took": time.Since(begin).String(),
		})
	})
	http.ListenAndServe(":8080", nil)
}

func (w openWeatherMap) temperature(city string) (float64, error) {
	resp, err := http.Get("http://api.openweathermap.org/data/2.5/weather?APPID=2323274f073784e2853487509b835880&q=" + city)
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()

	var d struct {
		Main struct {
			Kelvin float64 `json:"temp"`
		} `json:"main"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return 0, err
	}

	log.Printf("openWeatherMap:% s:% .2f", city, d.Main.Kelvin)

	return d.Main.Kelvin, nil
}

func (w weatherUnderground) temperature(city string) (float64, error) {
	resp, err := http.Get("http://api.wunderground.com/api/" + w.apiKey + "/conditions/q/" + city + ".json")
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()

	var d struct {
		Observation struct {
			Celsius float64 `json:"temp_c"`
		} `json:"current_observation"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return 0, err
	}

	kelvin := d.Observation.Celsius + 273.15

	log.Println("weatherUnderground:% s:% .2f", city, kelvin)
	
	return kelvin, nil
}

func temperature(city string, providers... weatherProvider) (float64, error) {
	sum := 0.00

	for _, provider := range providers {
		k, err := provider.temperature(city)

		if err != nil {
			return 0, nil
		}

		sum += k
	}

	return sum / float64(len(providers)), nil
}

func (w multiWheaterProvider) temperature(city string) (float64, error) {
	temps := make(chan float64, len(w))
	errs := make(chan error, len(w))

	for _, provider := range w {
		go func (p weatherProvider) {
			k, err := p.temperature(city)
			
			if err != nil {
				errs <- err
				return 
			}

			temps <- k
		}(provider)
	}

	sum := 0.0

	for i := 0; i < len(w); i++ {
		select {
		case temp := <- temps:
			sum += temp
		case err := <- errs:
			return 0, err
		}
	}

	/* for _, provider := range w {
		k, err := provider.temperature(city)

		if err != nil {
			return 0, err
		}	

		sum += k
	} */
	
	return sum / float64(len(w)), nil
}