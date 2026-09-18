package clients
import("crypto/aes";"crypto/cipher";"crypto/rand";"errors";"io")
type AESSecretBox struct{aead cipher.AEAD}
func NewAESSecretBox(key []byte)(*AESSecretBox,error){if len(key)!=32{return nil,errors.New("master key must be exactly 32 bytes")};block,err:=aes.NewCipher(key);if err!=nil{return nil,err};aead,err:=cipher.NewGCM(block);if err!=nil{return nil,err};return &AESSecretBox{aead:aead},nil}
func(b *AESSecretBox)Encrypt(plain []byte)([]byte,error){nonce:=make([]byte,b.aead.NonceSize());if _,err:=io.ReadFull(rand.Reader,nonce);err!=nil{return nil,err};return b.aead.Seal(nonce,nonce,plain,nil),nil}
func(b *AESSecretBox)Decrypt(ciphertext []byte)([]byte,error){if len(ciphertext)<b.aead.NonceSize(){return nil,errors.New("ciphertext is too short")};nonce:=ciphertext[:b.aead.NonceSize()];return b.aead.Open(nil,nonce,ciphertext[b.aead.NonceSize():],nil)}
