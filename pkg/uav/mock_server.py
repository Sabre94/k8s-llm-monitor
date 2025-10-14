#!/usr/bin/env python3
"""
UAV Mock Simulator
模拟无人机状态的HTTP服务器
"""

import os
import json
import time
from datetime import datetime
from http.server import BaseHTTPRequestHandler, HTTPServer

class UAVMockSimulator:
    def __init__(self, uav_id, node_name, gps_coords, battery_config, flight_config):
        self.uav_id = uav_id
        self.node_name = node_name
        self.gps_coords = gps_coords
        self.battery_config = battery_config
        self.flight_config = flight_config
        self.start_time = time.time()

    def get_current_state(self):
        """获取当前UAV状态，支持动态数据"""
        elapsed = time.time() - self.start_time

        # 动态GPS位置（微小移动模拟）
        lat_offset = 0.0001 * time.sin(elapsed * 0.1)
        lon_offset = 0.0001 * time.cos(elapsed * 0.1)

        # 动态电池电量（缓慢消耗）
        battery_drain = elapsed * 0.001  # 每秒消耗0.1%
        current_battery = max(20, self.battery_config['initial_percent'] - battery_drain)

        state = {
            "status": "success",
            "data": {
                "uav_id": self.uav_id,
                "node_name": self.node_name,
                "system_time": datetime.utcnow().isoformat() + "Z",
                "gps": {
                    "latitude": self.gps_coords['latitude'] + lat_offset,
                    "longitude": self.gps_coords['longitude'] + lon_offset,
                    "altitude": self.gps_coords['altitude'],
                    "satellite_count": self.gps_coords['satellite_count'],
                    "fix_type": self.gps_coords['fix_type']
                },
                "battery": {
                    "voltage": self.battery_config['voltage'],
                    "remaining_percent": round(current_battery, 1),
                    "temperature": self.battery_config['temperature']
                },
                "flight": {
                    "mode": self.flight_config['mode'],
                    "armed": self.flight_config['armed'],
                    "ground_speed": self.flight_config['ground_speed']
                },
                "health": {
                    "system_status": "OK" if current_battery > 30 else "WARNING"
                }
            }
        }
        return state

class UAVHandler(BaseHTTPRequestHandler):
    def __init__(self, simulator, *args, **kwargs):
        self.simulator = simulator
        super().__init__(*args, **kwargs)

    def do_GET(self):
        if self.path == '/health':
            self.send_health_response()
        elif self.path == '/api/v1/state':
            self.send_state_response()
        else:
            self.send_404()

    def send_health_response(self):
        self.send_response(200)
        self.send_header('Content-type', 'application/json')
        self.end_headers()
        response = {"status": "healthy", "uav_id": self.simulator.uav_id}
        self.wfile.write(json.dumps(response).encode())

    def send_state_response(self):
        self.send_response(200)
        self.send_header('Content-type', 'application/json')
        self.end_headers()
        state = self.simulator.get_current_state()
        self.wfile.write(json.dumps(state).encode())

    def send_404(self):
        self.send_response(404)
        self.end_headers()

    def log_message(self, format, *args):
        # 静默日志输出
        pass

def create_simulator_from_env():
    """从环境变量创建UAV模拟器"""
    uav_id = os.getenv('UAV_ID', 'UAV-UNKNOWN')
    node_name = os.getenv('NODE_NAME', 'unknown-node')

    # GPS配置
    gps_coords = {
        'latitude': float(os.getenv('GPS_LAT', '39.9042')),
        'longitude': float(os.getenv('GPS_LON', '116.4074')),
        'altitude': float(os.getenv('GPS_ALT', '50.0')),
        'satellite_count': int(os.getenv('GPS_SATS', '12')),
        'fix_type': int(os.getenv('GPS_FIX', '3'))
    }

    # 电池配置
    battery_config = {
        'voltage': float(os.getenv('BATT_VOLTAGE', '22.2')),
        'initial_percent': float(os.getenv('BATT_PERCENT', '85.0')),
        'temperature': float(os.getenv('BATT_TEMP', '28.0'))
    }

    # 飞行配置
    flight_config = {
        'mode': os.getenv('FLIGHT_MODE', 'AUTO'),
        'armed': os.getenv('FLIGHT_ARMED', 'true').lower() == 'true',
        'ground_speed': float(os.getenv('FLIGHT_SPEED', '5.0'))
    }

    return UAVMockSimulator(uav_id, node_name, gps_coords, battery_config, flight_config)

def main():
    # 从环境变量创建模拟器
    simulator = create_simulator_from_env()

    # 创建HTTP服务器
    def handler(*args, **kwargs):
        return UAVHandler(simulator, *args, **kwargs)

    server = HTTPServer(('0.0.0.0', 9090), handler)
    print(f"UAV Mock Simulator started for {simulator.uav_id} on node {simulator.node_name}")
    print("Listening on port 9090")
    print("Available endpoints:")
    print("  GET /health - Health check")
    print("  GET /api/v1/state - UAV state data")

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nShutting down server...")
        server.shutdown()

if __name__ == '__main__':
    main()