package main

/*
@author: TTG
@brief: websocket初始页内容, 写死嵌入在代码中,主要用作测试
*/

import (
	"fmt"
)

var IndexPageBuff []byte = []byte(fmt.Sprintf(`
<html>
<head>
    <meta charset="utf-8"/>
    <title>Websocket</title>
</head>
<body>
    <h1>测试WebSocket</h1>
    <form>
        <p>
            发送信息: <input id="content" type="text" placeholder="TTG_DEMO_0001" value="TTG_DEMO_0001" />
        </p>
    </form>
    <label id="result">接收信息：</label><br><br>
    <input type="button" name="button" id="button" value="提交" onclick="sk_send()"/>
    <script type="text/javascript">
        var sock = null;
        var domain_path = "%s:%s/websocket"
        
        var ishttps = 'https:' == document.location.protocol ? true: false;
        var wsuri = ishttps ? "wss://"+domain_path: "ws://"+domain_path;
        sock = new WebSocket(wsuri);
        
        sock.onmessage = function(e) {
            var result = document.getElementById('result');
            var text = (e.data.text && e.data.text()) || e.data;
            if(text.text){text = "BLOG, NEED FILEREADER TO DISERIES";}
            result.innerHTML = "收到回复：" + text;
        }
        
        sock.onclose = function(e){
          alert("websocket 连接断开， 请刷新页面[本页面host:ip]")
        }
            
        function sk_send() {
            var msg = document.getElementById('content').value;
            sock.send(msg);
        }
    </script>
</body>
</html>`, ConfigInstance.Host, ConfigInstance.Ports.WsPort))
