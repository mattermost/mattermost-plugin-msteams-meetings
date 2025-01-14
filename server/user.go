package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"

	"github.com/pkg/errors"
	msgraph "github.com/yaegashi/msgraph.go/beta"
	"golang.org/x/oauth2"
)

// UserInfo defines the information we store from each user
type UserInfo struct {
	Email string

	OAuthToken          *oauth2.Token `json:",omitempty"`
	EncryptedOAuthToken string        `json:",omitempty"`

	// Mattermost userID
	UserID string
	// Remote userID
	RemoteID string
	// Remote UPN
	UPN string
}

func DecryptUserInfo(data, key []byte) (*UserInfo, error) {
	i := UserInfo{}
	if err := json.Unmarshal(data, &i); err != nil {
		return nil, err
	}

	switch {
	case i.EncryptedOAuthToken == "":
		break
	case len(key) == 0:
		return nil, errors.New("decryption key required to access encrypted user Oauth2 token")
	default:
		decryptedData, err := decrypt(key, i.EncryptedOAuthToken)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decrypt user OAuth2 token")
		}

		t := oauth2.Token{}
		err = json.Unmarshal(decryptedData, &t)
		if err != nil {
			return nil, errors.Wrap(err, "failed to decode user OAuth2 token")
		}
		i.OAuthToken = &t
		i.EncryptedOAuthToken = ""
	}
	return &i, nil
}

func (i *UserInfo) EncryptedJSON(key []byte) ([]byte, error) {
	clone := *i
	clone.EncryptedOAuthToken = ""
	if len(key) != 0 {
		tokenData, err := json.Marshal(i.OAuthToken)
		if err != nil {
			return nil, errors.Wrap(err, "error occurred while encoding access token")
		}
		encryptedToken, err := encrypt(key, tokenData)
		if err != nil {
			return nil, errors.Wrap(err, "error occurred while encrypting access token")
		}
		clone.OAuthToken = nil
		clone.EncryptedOAuthToken = encryptedToken
	}
	return json.Marshal(clone)
}

const (
	tokenKey           = "token_"
	tokenKeyByRemoteID = "tbyrid_"
)

func (c *Client) GetMe() (*msgraph.User, error) {
	ctx := context.Background()
	graphUser, err := c.builder.Me().Request().Get(ctx)
	if err != nil {
		c.api.LogError(errors.Wrap(err, "cannot get user").Error())
		return nil, err
	}

	if graphUser == nil {
		err = errors.New("empty user")
		c.api.LogError(errors.Wrap(err, "cannot get user").Error())
		return nil, err
	}

	return graphUser, nil
}

func (p *Plugin) StoreUserInfo(info *UserInfo) error {
	key := []byte(p.getConfiguration().EncryptionKey)
	data, err := info.EncryptedJSON(key)
	if err != nil {
		return err
	}
	if appErr := p.API.KVSet(tokenKey+info.UserID, data); appErr != nil {
		return appErr
	}
	if appErr := p.API.KVSet(tokenKeyByRemoteID+info.RemoteID, data); appErr != nil {
		return appErr
	}
	return nil
}

func (p *Plugin) GetUserInfo(userID string) (*UserInfo, error) {
	infoBytes, appErr := p.API.KVGet(tokenKey + userID)
	if appErr != nil || infoBytes == nil {
		return nil, errors.New("Your Mattermost account is not connected to any Microsoft Teams account") //nolint:golint
	}

	key := []byte(p.getConfiguration().EncryptionKey)
	return DecryptUserInfo(infoBytes, key)
}

func (p *Plugin) RemoveUser(userID string) error {
	info, err := p.GetUserInfo(userID)
	if err != nil {
		return err
	}

	errByMattermostID := p.API.KVDelete(tokenKey + userID)
	errByRemoteID := p.API.KVDelete(tokenKeyByRemoteID + info.RemoteID)
	if errByMattermostID != nil {
		return errByMattermostID
	}
	if errByRemoteID != nil {
		return errByRemoteID
	}
	return nil
}

func encrypt(key, data []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", errors.Wrap(err, "could not create a cipher block, check key")
	}

	data = pad(data)
	ciphertext := make([]byte, aes.BlockSize+len(data))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", errors.Wrap(err, "readFull was unsuccessful, check buffer size")
	}

	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], data)
	finalMsg := base64.URLEncoding.EncodeToString(ciphertext)
	return finalMsg, nil
}

func decrypt(key []byte, text string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Wrap(err, "could not create a cipher block, check key")
	}

	decodedMsg, err := base64.URLEncoding.DecodeString(text)
	if err != nil {
		return nil, errors.Wrap(err, "could not decode the message")
	}

	if (len(decodedMsg) % aes.BlockSize) != 0 {
		return nil, errors.New("blocksize must be multiple of decoded message length")
	}

	iv := decodedMsg[:aes.BlockSize]
	msg := decodedMsg[aes.BlockSize:]

	cfb := cipher.NewCFBDecrypter(block, iv)
	cfb.XORKeyStream(msg, msg)

	unpadMsg, err := unpad(msg)
	if err != nil {
		return nil, errors.Wrap(err, "unpad error, check key")
	}

	return unpadMsg, nil
}

func pad(src []byte) []byte {
	padding := aes.BlockSize - len(src)%aes.BlockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(src, padtext...)
}

func unpad(src []byte) ([]byte, error) {
	length := len(src)
	unpadding := int(src[length-1])

	if unpadding > length {
		return nil, errors.New("unpad error. This could happen when incorrect encryption key is used")
	}

	return src[:(length - unpadding)], nil
}

func generateSecret() (string, error) {
	b := make([]byte, 256)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	s := base64.RawStdEncoding.EncodeToString(b)

	s = s[:32]

	return s, nil
}

func (p *Plugin) resetAllOAuthTokens() {
	// At this time the only data the plugin stores in the KV store is the user
	// OAuth2 tokens, and the temporary state to use during OAuth2
	// authentication flow. Since a change in the encryption key invalidates all
	// connections, we can safely remove all of plugin's data since it'll be
	// irrelevant anyway.
	p.API.LogInfo("OAuth2 configuration changed. Resetting all users' tokens, everyone will need to reconnect to MS Teams")
	appErr := p.API.KVDeleteAll()
	if appErr != nil {
		p.API.LogError("failed to reset user's OAuth2 tokens", "error", appErr.Error())
		return
	}
}
