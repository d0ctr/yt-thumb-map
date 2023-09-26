package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/dotenv-org/godotenvvault"
	"github.com/go-redis/redis/v8"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type Thumbnail struct {
	url    string `json:"url"`
	width  int    `json:"width"`
	height int    `json:"height"`
}

type YTVideoData struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Thumbnail     Thumbnail `json:"thumbnail"`
	DislikesCount uint64    `json:"dislikes"`
	LikesCount    uint64    `json:"likes"`
	ViewCount     uint64    `json:"views"`
	PublishedDate string    `json:"published_date"`
}

const STORAGE_PREFIX = "yt-data"

func genChannelHashKey(channelId string) string {
	return strings.Join([]string{STORAGE_PREFIX, channelId}, ":")
}

// Define your Redis connection details
var rdb *redis.Client

var httpPort string = "8080"

func init() {
	// load envs
	err := godotenvvault.Load("../.env")
	if err != nil {
		log.Fatal("Error loading .env files")
	}

	// Initialize Redis connection
	opts, err := redis.ParseURL(os.Getenv("REDIS_URL"))
	log.Printf("Parsed [%s] into: %v", os.Getenv("REDIS_URL"), opts)
	if err != nil {
		log.Fatalf("Failed to parse Redis URL: %v", err)
	}
	rdb = redis.NewClient(opts)

	if os.Getenv("PORT") != "" {
		httpPort = os.Getenv("PORT")
	}
}

func main() {
	ctx := context.Background()

	// Create a new YouTube service client
	service, err := youtube.NewService(ctx, option.WithAPIKey(os.Getenv("YOUTUBE_API_KEY")))
	if err != nil {
		log.Fatalf("Failed to create YouTube service client: %v", err)
	}

	// channelUserName := "@zavtracast" // Replace with the YouTube channel username you want to fetch data from

	// channelId := fetchChannel(ctx, service, channelUserName)
	channelId := "UCWu4IYWg9JpBurse8e-udPA"

	if channelId == "" {
		log.Fatal("Couldn't fetch channel's id")
	}

	Server := CreateServer(rdb, channelId)
	go Server.Start(httpPort)

	for {
		videoIdsChannel := make(chan []string)
		videosChannel := make(chan []YTVideoData)

		// fetch video ids till possible
		go func() {
			log.Print("Video Ids fetcher started")
			nextToken := ""
			var videoIds []string
			for {
				videoIds, nextToken = fetchVideoIds(ctx, service, channelId, nextToken)
				videoIdsChannel <- videoIds
				if nextToken == "" {
					break
				}
			}
			close(videoIdsChannel)
			log.Print("Video Ids fetcher finished")
		}()

		// fetch videos till new video ids exist
		go func() {
			log.Print("Videos fetcher started")
			for videoIds := range videoIdsChannel {
				videos := fetchVideos(ctx, service, videoIds)

				videosChannel <- videos
			}
			close(videosChannel)
			log.Print("Videos fetcher finished")
		}()

		// write to Reids till new videos exists
		go func() {
			log.Print("Redis saver started")
			for videos := range videosChannel {
				writeToRedis(ctx, channelId, videos)
			}
			log.Print("Redis saver finished")
		}()

		// Fetch new videos after every 1 hour
		log.Print("Sleeping...")
		time.Sleep(24 * time.Hour)
	}
}

func fetchChannel(ctx context.Context, service *youtube.Service, channelUserName string) string {
	// Make a request to retrieve the channel's id
	call := service.Search.List([]string{"id"}).
		Q(channelUserName).
		Type("channel").
		MaxResults(1)

	response, err := call.Do()
	if err != nil {
		log.Printf("Failed to find channel id for channel: %s", channelUserName)
		return ""
	}

	if len(response.Items) == 0 {
		log.Printf("No channels with such name: %s", channelUserName)
		return ""
	}
	channelId := response.Items[0].Id.ChannelId
	log.Printf("%s == %s", channelUserName, channelId)
	return channelId
}

func fetchVideoIds(ctx context.Context, service *youtube.Service, channelId string, nextToken string) ([]string, string) {
	var videoIds []string

	// Make a request to retrieve the channel's videos
	call := service.Search.List([]string{"id"}).
		ChannelId(channelId).
		Order("date").
		Type("video").
		MaxResults(50) // Number of videos to retrieve per request

	if nextToken != "" {
		call = call.PageToken(nextToken)
	}

	response, err := call.Do()

	if err != nil {
		log.Printf("Failed to fetch video ids for %s with token %s", channelId, nextToken)
		return videoIds, ""
	}

	for _, item := range response.Items {
		videoIds = append(videoIds, item.Id.VideoId)
	}

	log.Printf("Fetched %d ids for channel %s with nextToken %s", len(videoIds), channelId, nextToken)

	return videoIds, response.NextPageToken
}

func fetchVideos(ctx context.Context, service *youtube.Service, videoIds []string) []YTVideoData {
	var videos []YTVideoData

	// Fetch video details for all the video IDs
	response, err := service.Videos.List([]string{"id", "snippet", "statistics"}).
		Id(videoIds...).
		Do()

	if err != nil {
		log.Printf("Failed to fetch video details: %v", err)
		return videos
	}

	// Process the fetched videos
	for _, item := range response.Items {
		data := YTVideoData{
			ID:            item.Id,
			Title:         item.Snippet.Title,
			Thumbnail:     Thumbnail{
				url: item.Snippet.Thumbnails.Standard.Url,
				width: int(item.Snippet.Thumbnails.Standard.Width),
				height: int(item.Snippet.Thumbnails.Standard.Height),
			},
			LikesCount:    item.Statistics.LikeCount,
			DislikesCount: item.Statistics.DislikeCount,
			ViewCount:     item.Statistics.ViewCount,
			PublishedDate: item.Snippet.PublishedAt,
		}
		videos = append(videos, data)
	}

	log.Printf("Fetched %d videos", len(videos))

	return videos
}

func writeToRedis(ctx context.Context, channelId string, videos []YTVideoData) {
	ok := 0
	var wg sync.WaitGroup
	for _, video := range videos {
		data, err := json.Marshal(video)
		if err != nil {
			log.Printf("Failed to marshalise data for video %s: %v", video.ID, err)
			continue
		}

		publishedDate, err := time.ParseInLocation(time.RFC3339, video.PublishedDate, time.UTC)
		if err != nil {
			log.Printf("Failed to parse publish date for video %s: %v", video.ID, err)
			continue
		}

		wg.Add(1)
		ok++
		go func() {
			defer wg.Done()
			// Save the video meta info to Redis
			err = rdb.HSet(ctx, genChannelHashKey(channelId), publishedDate.Unix(), string(data)).Err()
			if err != nil {
				log.Printf("Failed to save video %s to Redis: %v", video.ID, err)
			}
		}()
	}
	wg.Wait()
	log.Printf("Wrote %d keys to redis", ok)
}
