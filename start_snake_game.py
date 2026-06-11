#!/usr/bin/env python3
"""
贪吃蛇游戏启动脚本
启动一个简单的HTTP服务器来运行贪吃蛇游戏
"""

import http.server
import socketserver
import webbrowser
import os

# 设置端口
PORT = 8000

# 设置目录为当前脚本所在目录
os.chdir(os.path.dirname(os.path.abspath(__file__)))

class MyHandler(http.server.SimpleHTTPRequestHandler):
    def end_headers(self):
        # 添加CORS头，防止跨域问题
        self.send_header('Access-Control-Allow-Origin', '*')
        super().end_headers()

def start_server():
    """启动HTTP服务器"""
    with socketserver.TCPServer(("", PORT), MyHandler) as httpd:
        print(f"🎮 贪吃蛇游戏启动成功！")
        print(f"🌐 访问地址: http://localhost:{PORT}/snake_game.html")
        print(f"📂 当前目录: {os.getcwd()}")
        print(f"⏹️  按 Ctrl+C 停止服务器")
        
        # 自动打开浏览器
        try:
            webbrowser.open(f"http://localhost:{PORT}/snake_game.html")
        except:
            pass
        
        try:
            httpd.serve_forever()
        except KeyboardInterrupt:
            print("\n🛑 服务器已停止")
            httpd.shutdown()

if __name__ == "__main__":
    start_server()