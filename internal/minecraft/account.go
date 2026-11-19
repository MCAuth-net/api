package minecraft

import (
	"MCAuth-api/internal/helpers"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

type Profile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func GetName(uuid string, rdb *redis.Client) (string, error) {
	ctx, cancel := helpers.GetContext()
	defer cancel()

	cache, err := rdb.Get(ctx, fmt.Sprintf("name:%s", uuid)).Result()
	if err == nil {
		return cache, nil
	}

	res, err := http.Get(fmt.Sprintf("https://api.mojang.com/user/profile/%s", uuid))
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", errors.New(res.Status)
	}

	var profile Profile
	if err := json.NewDecoder(res.Body).Decode(&profile); err != nil {
		return "", err
	}

	err = rdb.Set(ctx, fmt.Sprintf("name:%s", uuid), profile.Name, 24*time.Hour).Err()

	return profile.Name, err
}
