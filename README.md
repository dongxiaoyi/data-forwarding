# 说明文档

> 基于https://github.com/childe/gohangout  做了点定制,具体操作请参见作者原项目。

增加：
- output插件kafka：可使用账号密码连接。
- output插件tcp：使用压缩后加密传输。
- input插件tcp：加密数据解密后解压缩。


## Example：
1. output -> kafka:
```yaml
outputs:
    - Kafka:
        topic: test-topic
        bootstrap.servers: "192.168.75.111:9092,192.168.75.112:9092,192.168.75.113:9092"
        producer_settings:
            sasl.mechanism: PLAIN
            sasl.user: testuser
            sasl.password: testpass
```

2. output -> tcp:
```yaml
outputs:
    - TCP:
        network: tcp4
        address: 192.168.75.114:10000
        concurrent: 1
        dial.timeout: 5
        max_length: 4096
```

3. input -> tcp:
```yaml
inputs:
    - TCP:
        network: tcp4
        address: 192.168.75.115:10001
        codec: plain
        max_length: 4096
```