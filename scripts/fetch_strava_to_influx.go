package main

import (
    "context"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "net/http"
    "net/url"
    "os"
    "strconv"
    "time"

    influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

type Activity struct {
    ID           int64   `json:"id"`
    Name         string  `json:"name"`
    StartDate    string  `json:"start_date"`
    Distance     float64 `json:"distance"`
    MovingTime   int     `json:"moving_time"`
    ElapsedTime  int     `json:"elapsed_time"`
    TotalElevationGain float64 `json:"total_elevation_gain"`
    Type         string  `json:"type"`
    AverageSpeed float64 `json:"average_speed"`
    MaxSpeed     float64 `json:"max_speed"`
    AverageWatts float64 `json:"average_watts"`
    Kilojoules   float64 `json:"kilojoules"`
}

func fetchStravaActivities(accessToken string, after, before int64) ([]Activity, error) {
    apiUrl := "https://www.strava.com/api/v3/athlete/activities"
    params := url.Values{}
    params.Set("after", strconv.FormatInt(after, 10))
    params.Set("before", strconv.FormatInt(before, 10))
    params.Set("per_page", "200")

    req, err := http.NewRequest("GET", apiUrl+"?"+params.Encode(), nil)
    if err != nil {
        return nil, err
    }
    req.Header.Set("Authorization", "Bearer "+accessToken)

    client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    var activities []Activity
    if err := json.Unmarshal(body, &activities); err != nil {
        return nil, err
    }
    return activities, nil
}

func writeToInflux(activities []Activity, influxUrl, token, org, bucket string) {
    client := influxdb2.NewClient(influxUrl, token)
    writeAPI := client.WriteAPIBlocking(org, bucket)

    for _, act := range activities {
        t, _ := time.Parse(time.RFC3339, act.StartDate)
        p := influxdb2.NewPoint("activity",
            map[string]string{"type": act.Type, "name": act.Name},
            map[string]interface{}{
                "distance":      act.Distance,
                "moving_time":   act.MovingTime,
                "elapsed_time":  act.ElapsedTime,
                "elevation":     act.TotalElevationGain,
                "avg_speed":     act.AverageSpeed,
                "max_speed":     act.MaxSpeed,
                "avg_watts":     act.AverageWatts,
                "kilojoules":    act.Kilojoules,
            },
            t)
        if err := writeAPI.WritePoint(context.Background(), p); err != nil {
            log.Printf("failed to write activity %d: %v", act.ID, err)
        }
    }

    client.Close()
}

func main() {
    if len(os.Args) < 3 {
        log.Fatalf("Usage: %s <after YYYY-MM-DD> <before YYYY-MM-DD>", os.Args[0])
    }

    afterStr, beforeStr := os.Args[1], os.Args[2]
    after, _ := time.Parse("2006-01-02", afterStr)
    before, _ := time.Parse("2006-01-02", beforeStr)

    // Load env vars
    accessToken := os.Getenv("STRAVA_ACCESS_TOKEN")
    influxURL := os.Getenv("INFLUXDB_URL")
    influxToken := os.Getenv("INFLUXDB_TOKEN")
    influxOrg := os.Getenv("INFLUXDB_ORG")
    influxBucket := os.Getenv("INFLUXDB_BUCKET")

    if accessToken == "" || influxToken == "" || influxURL == "" || influxOrg == "" || influxBucket == "" {
        log.Fatal("Required environment variables are missing")
    }

    activities, err := fetchStravaActivities(accessToken, after.Unix(), before.Unix())
    if err != nil {
        log.Fatalf("Error fetching activities: %v", err)
    }

    fmt.Printf("Fetched %d activities\n", len(activities))
    writeToInflux(activities, influxURL, influxToken, influxOrg, influxBucket)
}
