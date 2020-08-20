package output

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/des"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/childe/gohangout/simplejson"
	"github.com/golang/glog"
	"github.com/pierrec/lz4"
)

type TCPOutput struct {
	config    map[interface{}]interface{}
	network   string
	address   string
	timeout   time.Duration
	keepalive time.Duration

	concurrent int
	messages   chan map[string]interface{}
	conn       []net.Conn
	//writer *bufio.Writer

	dialLock sync.Mutex
}

func (l *MethodLibrary) NewTCPOutput(config map[interface{}]interface{}) *TCPOutput {
	p := &TCPOutput{
		config:     config,
		concurrent: 1,
	}

	p.network = "tcp"
	if network, ok := config["network"]; ok {
		p.network = network.(string)
	}

	if addr, ok := config["address"]; ok {
		p.address, ok = addr.(string)
	} else {
		glog.Fatal("address must be set in TCP output")
	}

	if timeoutI, ok := config["dial.timeout"]; ok {
		timeout := timeoutI.(int)
		p.timeout = time.Second * time.Duration(timeout)
	}

	if keepaliveI, ok := config["keepalive"]; ok {
		keepalive, ok := keepaliveI.(int)
		if !ok {
			glog.Fatal("keepalive must be integer")
		}
		p.keepalive = time.Second * time.Duration(keepalive)
	}

	if v, ok := config["concurrent"]; ok {
		p.concurrent = v.(int)
	}
	p.messages = make(chan map[string]interface{}, p.concurrent)
	p.conn = make([]net.Conn, p.concurrent)

	for i := 0; i < p.concurrent; i++ {
		go func(i int) {
			p.conn[i] = p.loopDial()
			for {
				// p.conn[i] = p.loopDial()
				event := <-p.messages
				d := &simplejson.SimpleJsonDecoder{}
				buf, err := d.Encode(event)
				if err != nil {
					glog.Errorf("marshal %v error:%s", event, err)
					return
				}

				// 对buf进行压缩
				newData := make([]byte, len(buf))
				ht := make([]int, 1<<22)

				n, err := lz4.CompressBlock(buf, newData, ht)
				if err != nil {
					fmt.Println(err)
				}

				// 加密
				var cryptText []byte
				if n != 0 {
					// newData = newData[:n]
					// cryptText, err = goEncrypt.AesCbcEncrypt(newData, []byte("wumansgygoaescry"))
					// if err != nil {
					// 	fmt.Println(err)
					// }
					cryptText = newData[:n]
				} else {
					cryptText = buf
				}

				key := []byte("passphrasewhichn")
				c, err := aes.NewCipher(key)
				if err != nil {
					fmt.Println(err)
				}

				gcm, err := cipher.NewGCM(c)
				if err != nil {
					fmt.Println(err)
				}

				nonce := make([]byte, gcm.NonceSize())
				if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
					fmt.Println(err)
				}
				cryptText = gcm.Seal(nonce, nonce, cryptText, nil)

				newData = append([]byte(cryptText), []byte("!!@@##$$")...)
				for {
					if err = write(p.conn[i], newData); err != nil {
						// glog.Error(err)
						p.conn[i].Close()
						p.conn[i] = p.loopDial()
					} else {
						// p.conn[i].Close()
						// p.conn[i] = p.loopDial()
						break
					}
				}
			}
		}(i)
	}

	return p
}

func (p *TCPOutput) loopDial() net.Conn {
	for {
		if conn, err := p.dial(); err != nil {
			glog.Errorf("dial error: %s. sleep 1s", err)
			time.Sleep(1 * time.Second)
		} else {
			glog.Infof("conn built to %s", conn.RemoteAddr())
			return conn
		}
	}
}

func (p *TCPOutput) dial() (net.Conn, error) {
	var d net.Dialer
	d.Timeout = p.timeout
	d.KeepAlive = p.keepalive

	conn, err := net.Dial(p.network, p.address)
	if err != nil {
		return conn, err
	}
	// *TcpConn is net.Conn interface, so we can pass conn instead of &conn
	go probe(conn)
	//p.writer = bufio.NewWriter(conn)

	return conn, nil
}

func probe(conn net.Conn) {
	var b = make([]byte, 1)

	conn.SetDeadline(time.Time{})
	conn.SetReadDeadline(time.Time{})
	_, err := conn.Read(b) // should block here
	if err != nil && err == io.EOF {
		glog.Infof("conn [%s] is closed by the server, close the conn.", conn.RemoteAddr())
		conn.Close()
	}
}

func (p *TCPOutput) Emit(event map[string]interface{}) {
	p.messages <- event
	//buf = append(buf, '\n')
	//n, err := p.writer.Write(buf)
	//if n != len(buf) {
	//glog.Errorf("write to %s[%s] error: %s", p.address, p.conn.RemoteAddr(), err)
	//}
	//p.writer.Flush()
}

func write(conn net.Conn, buf []byte) error {
	for len(buf) > 0 {
		n, err := conn.Write(buf)
		if err != nil {
			return err
			//glog.Errorf("write to %s[%s] error: %s", p.address, conn.RemoteAddr(), err)
			//switch {
			//case strings.Contains(str, "use of closed network connection"):
			//conn = loopDial()
			//return err
			//case strings.Contains(str, "write: broken pipe"):
			//conn.Close()
			//conn = loopDial()
			//return err
			//}
		}
		buf = buf[n:]
	}
	return nil
}

func (p *TCPOutput) Shutdown() {
	//p.writer.Flush()
	//p.conn.Close()
}

func EncryptByDESAndCBC(text string, key string, iv string) []byte {

	textBytes := []byte(text)
	keyBytes := []byte(key)
	ivBytes := []byte(iv)

	//加密
	//1. 创建并返回一个使用DES算法的cipher.Block接口
	//使用des调用NewCipher获取block接口
	block, err := des.NewCipher(keyBytes)
	if err != nil {
		panic(err)
	}

	//2. 填充数据，将输入的明文构造成8的倍数
	textBytes = MakeBlocksFull(textBytes, 8)

	//3. 创建CBC分组模式,返回一个密码分组链接模式的、底层用b加密的BlockMode接口，初始向量iv的长度必须等于b的块尺寸
	//使用cipher调用NewCBCDecrypter获取blockMode接口
	blockMode := cipher.NewCBCEncrypter(block, ivBytes)

	//3. 加密
	//这里的两个参数为什么都是textBytes
	//第一个是目标,第二个是源
	//也就是说将第二个进行加密,然后放到第一个里面
	//如果我们重新定义一个密文cipherTextBytes
	//那么就是blockMode.CryptBlocks(cipherTextBytes, textBytes)
	blockMode.CryptBlocks(textBytes, textBytes)

	return textBytes

}

func MakeBlocksFull(src []byte, blockSize int) []byte {

	//1. 获取src的长度， blockSize对于des是8
	length := len(src)

	//2. 对blockSize进行取余数， 4
	remains := length % blockSize

	//3. 获取要填的数量 = blockSize - 余数
	paddingNumber := blockSize - remains //4

	//4. 将填充的数字转换成字符， 4， '4'， 创建了只有一个字符的切片
	//s1 = []byte{'4'}
	s1 := []byte{byte(paddingNumber)}

	//5. 创造一个有4个'4'的切片
	//s2 = []byte{'4', '4', '4', '4'}
	s2 := bytes.Repeat(s1, paddingNumber)

	//6. 将填充的切片追加到src后面
	s3 := append(src, s2...)

	return s3
}
