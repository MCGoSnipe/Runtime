package runtime

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	//MinecraftServicesAPIHost The hostname for Minecraft's API.
	MinecraftServicesAPIHost = "api.minecraftservices.com"
	//YggdrasilAuthURI The URI for Mojang's authentication server.
	YggdrasilAuthURI = "https://authserver.mojang.com"
	//MicrosoftLoginAPI The URI for logging in to a Microsoft account for Minecraft via MicroAuth.
	MicrosoftLoginAPI = "https://login.live.com/oauth20_authorize.srf?client_id=9abe16f4-930f-4033-b593-6e934115122f&response_type=code&redirect_uri=https%3A%2F%2Fmicroauth.tk%2Ftoken&scope=XboxLive.signin%20XboxLive.offline_access"
)

//Configuration This struct holds configuration for
// the sniper to use.
type Configuration struct {
	Bearer    string
	Name      string
	Offset    float64
	Timestamp time.Time
	Label     *string
}

//SnipeRes This contains the data obtained after the snipe.
type SnipeRes struct {
	Sent   *time.Time
	Recv   *time.Time
	Status *int
	Label  *string
}

// Internal structs here

type nxAPIRes struct {
	DropTime string `json:"drop_time"`
}

type securityRes struct {
	Answer answerRes `json:"answer"`
}

type answerRes struct {
	ID int `json:"id"`
}

type msaRes struct {
	AccessToken *string `json:"access_token"`
	MSAError    *string `json:"error"`
}

type yggAgent struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

type accessTokenRequest struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	Agent    yggAgent `json:"agent"`
}

type accessTokenResponse struct {
	AccessToken *string `json:"access_token"`
	YggError    *string `json:"error"`
}

//TextToSliceStr This function will return a slice
// of all the lines of the file, found at path, as strings.
// Also returns the number of lines read.
func TextToSliceStr(path string) ([]string, int) {
	file, err := os.Open("./accounts.txt")
	i := 0
	if err == nil {
		var txtSlice []string
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "\n") {
				line = line[:len(line)-1]
			}
			if strings.Contains(line, "\r") {
				line = line[:len(line)-1]
			}
			txtSlice = append(txtSlice, scanner.Text())
			i++
		}
		return txtSlice, i
	}
	return make([]string, 0), 0
}

//SliceStrToBearers Returns a list of bearers from the input provided alongside the count of bearers.
// This function also will automatically answer security questions if provided.
func SliceStrToBearers(inputSlice []string) ([]string, int) {
	outputSlice := make([]string, 0)
	i := 0
	client := &http.Client{}
	for _, input := range inputSlice {
		splitLogin := strings.Split(input, ":")
		data := accessTokenRequest{
			Agent: yggAgent{
				Name:    "Minecraft",
				Version: 1,
			},
			Username: splitLogin[0],
			Password: splitLogin[1],
		}
		bytesToSend, err := json.Marshal(data)
		if err != nil {
			continue
		}
		req, err := http.NewRequest("POST", YggdrasilAuthURI+"/authenticate", bytes.NewBuffer(bytesToSend))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "GoSnipe/2.0")
		if err != nil {
			continue
		}
		res, err := client.Do(req)
		if err != nil {
			continue
		}
		if res.Status != "200 OK" {
			continue
		}
		respData, err := ioutil.ReadAll(res.Body)
		if err != nil {
			continue
		}
		var access accessTokenResponse
		err = json.Unmarshal(respData, &access)
		if err != nil {
			continue
		}
		outputSlice = append(outputSlice, *access.AccessToken)
		i++
		if len(splitLogin) == 5 {
			req, err = http.NewRequest("GET", "https://api.mojang.com/user/security/challenges", nil)
			if err != nil {
				continue
			}
			req.Header.Set("Authorization", "Bearer "+*access.AccessToken)
			res, err = client.Do(req)
			if err != nil {
				continue
			}
			respData, err = ioutil.ReadAll(res.Body)
			if err != nil {
				continue
			}
			var security []securityRes
			err = json.Unmarshal(respData, &security)
			if err != nil {
				continue
			}
			dataBytes := []byte(`[{"id": ` + strconv.Itoa(security[0].Answer.ID) + `, "answer": "` + splitLogin[2] + `"}, {"id": ` + strconv.Itoa(security[1].Answer.ID) + `, "answer": "` + splitLogin[3] + `"}, {"id": ` + strconv.Itoa(security[2].Answer.ID) + `, "answer": "` + splitLogin[4] + `"}]`)
			req, err = http.NewRequest("POST", "https://api.mojang.com/user/security/location", bytes.NewReader(dataBytes))
			if err != nil {
				continue
			}
			req.Header.Set("Authorization", "Bearer "+*access.AccessToken)
			client.Do(req)
		}
	}
	return outputSlice, i
}

//GetDropTime Gets the time.Time of when the inputted name drops. Returns nil upon error.
func GetDropTime(name string) *time.Time {
	res, err := http.Get("https://api.nathan.cx/check/" + name)
	if err != nil {
		return nil
	}
	apiRes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil
	}
	var nxres nxAPIRes
	res.Body.Close()
	json.Unmarshal(apiRes, &nxres)
	timestamp, err := time.Parse(time.RFC3339, nxres.DropTime)
	if err != nil {
		return nil
	}
	return &timestamp
}

//AutoOffset Returns a floating-point offset based on connection speed or nil.
func AutoOffset(count ...int) *float64 {
	c := 3
	if len(count) > 0 {
		c = count[0]
	}
	payload := "PUT /minecraft/profile/name/test HTTP/1.1\r\nHost: api.minecraftservices.com\r\nAuthorization: Bearer TestToken" + "\r\n\r\n"
	conn, err := tls.Dial("tls", MinecraftServicesAPIHost+":443", nil)
	if err != nil {
		return nil
	}
	sumNanos := int64(0)
	for i := 0; i < c; i++ {
		junk := make([]byte, 4096)
		time1 := time.Now()
		conn.Write([]byte(payload))
		conn.Read(junk)
		duration := time.Now().Sub(time1)
		sumNanos += duration.Nanoseconds()
	}
	avgMillis := float64(sumNanos) / float64(1000000)
	return &avgMillis
}

//Snipe Snipes with config and sends response through the channel given.
func Snipe(config Configuration, ch chan SnipeRes) {
	time.Sleep(time.Until(config.Timestamp.Add(time.Millisecond * time.Duration(0-10000-config.Offset))))
	recvd := make([]byte, 4096)
	conn, err := tls.Dial("tls", MinecraftServicesAPIHost+":443", nil)
	if err != nil {
		ch <- SnipeRes{}
	}
	payload := "PUT /minecraft/profile/name/" + config.Name + " HTTP/1.1\r\nHost: api.minecraftservices.com\r\nAuthorization: Bearer " + config.Bearer + "\r\n"
	conn.Write([]byte(payload))
	time.Sleep(time.Until(config.Timestamp))
	conn.Write([]byte("\r\n"))
	sent := time.Now()
	conn.Read(recvd)
	recv := time.Now()
	code, _ := strconv.Atoi(string(recvd[9:12]))
	ch <- SnipeRes{
		Sent:   &sent,
		Recv:   &recv,
		Status: &code,
		Label:  config.Label,
	}
}