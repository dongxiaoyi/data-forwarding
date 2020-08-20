package input

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"fmt"
	"io"
	"net"

	"github.com/childe/gohangout/codec"
	"github.com/golang/glog"
	"github.com/pierrec/lz4"
)

type TCPInput struct {
	config  map[interface{}]interface{}
	network string
	address string

	decoder codec.Decoder

	l        net.Listener
	messages chan []byte
	stop     bool
}

func readLine(c net.Conn, messages chan<- []byte, maxLength int) {
	buf := make([]byte, 0, maxLength)
	tmp := make([]byte, 256)

	for {
		data := make([]byte, 0)
	loop:
		for {
			for {
				n, err := c.Read(tmp)
				if err != nil {
					if err != io.EOF {
						panic("读取错误")
					}
					break
				}
				//fmt.Println("got", n, "bytes.")
				buf = append(buf, tmp[:n]...)

				if bytes.Contains(buf, []byte("!!@@##$$")) {
					t := bytes.Split(buf, []byte("!!@@##$$"))
					data = t[0]
					t2 := make([]byte, 0)
					for i := 1; i < len(t); i++ {
						t2 = append(t2, t[i]...)
					}
					buf = t2
					break loop
				}
			}
		}
		defaultBufLen := 10
		// data = bytes.TrimRight(buf, "!!@@##$$")
		if len(data) != 0 {
			for {
				// 解密
				// newText, err := goEncrypt.AesCbcDecrypt(buf, []byte("wumansgygoaescry"))
				// if err != nil {
				// 	fmt.Println(err)
				// }
				key := []byte("passphrasewhichn")
				c, err := aes.NewCipher(key)
				if err != nil {
					fmt.Println(err)
				}

				gcm, err := cipher.NewGCM(c)
				if err != nil {
					fmt.Println(err)
				}

				nonceSize := gcm.NonceSize()
				if len(data) < nonceSize {
					fmt.Println(err)
				}

				nonce, ciphertext := data[:nonceSize], data[nonceSize:]
				newText, err := gcm.Open(nil, nonce, ciphertext, nil)
				if err != nil {
					fmt.Println(err)
				}
				out := make([]byte, defaultBufLen*len(newText))
				n, err := lz4.UncompressBlock(newText, out)

				if n != 0 {
					if err != nil {
						defaultBufLen++
						continue
					} else {
						out = out[:n]
					}
				} else {
					out = newText
				}
				messages <- out

			}
		}
		// c.Close()
		// close(messages)
	}

}

func (lib *MethodLibrary) NewTCPInput(config map[interface{}]interface{}) *TCPInput {
	var codertype string = "plain"
	if v, ok := config["codec"]; ok {
		codertype = v.(string)
	}

	p := &TCPInput{
		config:   config,
		decoder:  codec.NewDecoder(codertype),
		messages: make(chan []byte, 10),
	}

	if v, ok := config["max_length"]; ok {
		if max, ok := v.(int); ok {
			if max <= 0 {
				glog.Fatal("max_length must be bigger than zero")
			}
		} else {
			glog.Fatal("max_length must be int")
		}
	}

	p.network = "tcp"
	if network, ok := config["network"]; ok {
		p.network = network.(string)
	}

	if addr, ok := config["address"]; ok {
		p.address = addr.(string)
	} else {
		glog.Fatal("address must be set in TCP input")
	}

	l, err := net.Listen(p.network, p.address)
	if err != nil {
		glog.Fatal(err)
	}
	p.l = l

	go func() {

		for !p.stop {
			conn, err := l.Accept()
			if err != nil {
				if p.stop {
					return
				}
				glog.Error(err)
			} else {
				var maxLength int
				// scanner := bufio.NewScanner(conn)
				if v, ok := config["max_length"]; ok {
					maxLength = v.(int)
				} else {
					maxLength = 4096
				}
				go readLine(conn, p.messages, maxLength)
			}
		}
	}()
	return p
}

func (p *TCPInput) ReadOneEvent() map[string]interface{} {
	text, more := <-p.messages
	if !more || text == nil {
		return nil
	}

	return p.decoder.Decode(text)
}

func (p *TCPInput) Shutdown() {
	p.stop = true
	p.l.Close()
	// close(p.messages)
}

func DecryptByDESAndCBC(cipherText []byte, key, iv string) string {

	textBytes := cipherText
	keyBytes := []byte(key)
	ivBytes := []byte(iv)

	//1. 创建并返回一个使用DES算法的cipher.Block接口。
	block, err := des.NewCipher(keyBytes)
	if err != nil {
		panic(err)
	}

	//2. 创建CBC分组模式
	blockMode := cipher.NewCBCDecrypter(block, ivBytes)

	//3. 解密
	blockMode.CryptBlocks(textBytes, textBytes)

	//4. 去掉填充数据 (注意去掉填充的顺序是在解密之后)
	plainText := MakeBlocksOrigin(textBytes)

	return string(plainText)
}

func MakeBlocksOrigin(src []byte) []byte {

	//1. 获取src长度
	length := len(src)

	//2. 得到最后一个字符
	lastChar := src[length-1] //'4'

	//3. 将字符转换为数字
	number := int(lastChar) //4

	//4. 截取需要的长度
	return src[:length-number]
}
