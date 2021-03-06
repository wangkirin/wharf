package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/satori/go.uuid"

	"github.com/containerops/wharf/utils"
)

type Image struct {
	Id         string   `json:"id"`         //
	ImageId    string   `json:"imageid"`    //
	JSON       string   `json:"json"`       //
	Ancestry   string   `json:"ancestry"`   //
	Checksum   string   `json:"checksum"`   // tarsum+sha256
	Payload    string   `json:"payload"`    //
	URL        string   `json:"url"`        //
	Backend    string   `json:"backend"`    //
	Path       string   `json:"path"`       //
	Sign       string   `json:"sign"`       //
	Size       int64    `json:"size"`       //
	Uploaded   bool     `json:"uploaded"`   //
	Checksumed bool     `json:"checksumed"` //
	Encrypted  bool     `json:"encrypted"`  //
	Version    int64    `json:"version"`    //
	Created    int64    `json:"created"`    //
	Updated    int64    `json:"updated"`    //
	Memo       []string `json:"memo"`       //
}

func (i *Image) Has(image string) (bool, []byte, error) {
	id, err := GetByGobalId("image", image)
	if err != nil {
		return false, nil, err
	}
	if len(id) <= 0 {
		return false, nil, nil
	}

	err = Get(i, id)

	return true, id, err
}

func (i *Image) HasTarsum(tarsum string) (bool, []byte, error) {
	id, err := GetByGobalId("tarsum", tarsum)
	if err != nil {
		return false, nil, err
	}
	if len(id) <= 0 {
		return false, nil, nil
	}

	err = Get(i, id)

	return true, id, err
}

func (i *Image) Save() error {
	if err := Save(i, []byte(i.Id)); err != nil {
		return err
	}

	if _, err := LedisDB.HSet([]byte(GLOBAL_IMAGE_INDEX), []byte(i.ImageId), []byte(i.Id)); err != nil {
		return err
	}

	return nil
}

func (i *Image) Get(id string) error {
	if err := Get(i, []byte(id)); err != nil {
		return err
	}

	return nil
}

func (i *Image) Remove() (err error) {
	if _, err := LedisDB.HSet([]byte(fmt.Sprintf("%s_remove", GLOBAL_IMAGE_INDEX)), []byte(i.ImageId), []byte(i.Id)); err != nil {
		return err
	}

	if _, err := LedisDB.HDel([]byte(GLOBAL_IMAGE_INDEX), []byte(i.Id)); err != nil {
		return err
	}

	return nil
}

func (i *Image) Pushed(imageId string) (bool, error) {
	if has, _, err := i.Has(imageId); err != nil {
		return false, err
	} else if has == false {
		return false, fmt.Errorf("Image not found")
	} else if i.Checksumed && i.Uploaded {
		return true, nil
	}

	return false, nil
}

func (i *Image) GetJSON(imageId string) ([]byte, error) {
	if has, _, err := i.Has(imageId); err != nil {
		return nil, err
	} else if has == false {
		return nil, fmt.Errorf("Image not found")
	} else if !i.Checksumed || !i.Uploaded {
		return nil, fmt.Errorf("Image JSON not found")
	} else {
		return []byte(i.JSON), nil
	}
}

func (i *Image) GetChecksum(imageId string) ([]byte, error) {
	if has, _, err := i.Has(imageId); err != nil {
		return nil, err
	} else if has == false {
		return nil, fmt.Errorf("Image not found")
	} else if !i.Checksumed || !i.Uploaded {
		return nil, fmt.Errorf("Image JSON not found")
	} else {
		return []byte(i.Checksum), nil
	}
}

func (i *Image) PutJSON(imageId, json string, version int64) error {
	if has, _, err := i.Has(imageId); err != nil {
		return err
	} else if has == false {
		i.Id, i.ImageId, i.JSON, i.Created, i.Version = string(utils.GeneralKey(uuid.NewV4().String())), imageId, json, time.Now().UnixNano()/int64(time.Millisecond), version

		if err = i.Save(); err != nil {
			return err
		}
	} else {
		i.ImageId, i.JSON = imageId, json
		i.Uploaded, i.Checksumed, i.Encrypted, i.Size, i.Updated, i.Version = false, false, false, 0, time.Now().UnixNano()/int64(time.Millisecond), version

		if err := i.Save(); err != nil {
			return err
		}
	}

	return nil
}

func (i *Image) PutLayer(imageId string, path string, uploaded bool, size int64) error {
	if has, _, err := i.Has(imageId); err != nil {
		return err
	} else if has == false {
		return fmt.Errorf("Image not found")
	} else {
		i.Path, i.Uploaded, i.Size, i.Updated = path, uploaded, size, time.Now().UnixNano()/int64(time.Millisecond)

		if err := i.Save(); err != nil {
			return err
		}
	}

	return nil
}

func (i *Image) PutChecksum(imageId string, checksum string, checksumed bool, payload string) error {
	if has, _, err := i.Has(imageId); err != nil {
		return err
	} else if has == false {
		return fmt.Errorf("Image not found")
	} else {
		if err := i.PutAncestry(imageId); err != nil {

			return err
		}

		i.Checksum, i.Checksumed, i.Payload, i.Updated = checksum, checksumed, payload, time.Now().UnixNano()/int64(time.Millisecond)

		if err = i.Save(); err != nil {
			return err
		}

		//Add checksum for V2 image index
		if _, err := LedisDB.HSet([]byte(GLOBAL_TARSUM_INDEX), []byte(checksum), []byte(i.Id)); err != nil {
			return err
		}
	}

	return nil
}

func (i *Image) PutAncestry(imageId string) error {
	if has, _, err := i.Has(imageId); err != nil {
		return err
	} else if has == false {
		return fmt.Errorf("Image not found")
	}

	var imageJSONMap map[string]interface{}
	var imageAncestry []string

	if err := json.Unmarshal([]byte(i.JSON), &imageJSONMap); err != nil {
		return err
	}

	if value, has := imageJSONMap["parent"]; has == true {
		parentImage := new(Image)
		parentHas, _, err := parentImage.Has(value.(string))
		if err != nil {
			return err
		}

		if !parentHas {
			return fmt.Errorf("Parent image not found")
		}

		var parentAncestry []string
		json.Unmarshal([]byte(parentImage.Ancestry), &parentAncestry)
		imageAncestry = append(imageAncestry, imageId)
		imageAncestry = append(imageAncestry, parentAncestry...)
	} else {
		imageAncestry = append(imageAncestry, imageId)
	}

	ancestryJSON, _ := json.Marshal(imageAncestry)
	i.Ancestry = string(ancestryJSON)

	if err := i.Save(); err != nil {
		return err
	}

	return nil
}

func (image *Image) Log(action, level, t int64, actionId string, content []byte) error {
	log := Log{Action: action, ActionId: actionId, Level: level, Type: t, Content: string(content), Created: time.Now().UnixNano() / int64(time.Millisecond)}
	log.Id = string(utils.GeneralKey(actionId))

	if err := log.Save(); err != nil {
		return err
	}

	image.Memo = append(image.Memo, log.Id)

	if err := image.Save(); err != nil {
		return err
	}

	return nil
}
