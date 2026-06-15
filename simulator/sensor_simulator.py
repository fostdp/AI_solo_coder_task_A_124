import requests
import time
import random
import math
import json
from datetime import datetime, timezone
from typing import List, Dict

API_BASE_URL = "http://localhost:8080/api"

class DouGateSimulator:
    def __init__(self, gate_id: int, gate_config: Dict):
        self.gate_id = gate_id
        self.gate_config = gate_config
        self.water_level_up = gate_config.get("max_water_level_up", 8.0) - 0.5
        self.water_level_down = gate_config.get("min_water_level_down", 3.0) + 0.5
        self.gate_opening = 0.0
        self.flow_rate = 0.0
        self.passage_time = 0.0
        self.status = "normal"
        self.is_charging = False
        self.is_discharging = False
        self.phase = "idle"
        self.phase_start_time = time.time()
        self.passage_count = 0

    def _calculate_flow_rate(self):
        head_diff = self.water_level_up - self.water_level_down
        if abs(head_diff) < 0.01 or self.gate_opening < 0.01:
            return 0.0

        Cd = self.gate_config.get("discharge_coefficient", 0.63)
        gate_width = self.gate_config.get("gate_width", 6.0)
        gate_height = self.gate_config.get("gate_height", 4.5)
        opening_height = self.gate_opening * gate_height
        g = 9.81

        if head_diff > 0:
            flow = Cd * opening_height * gate_width * math.sqrt(2 * g * head_diff)
        else:
            flow = -Cd * opening_height * gate_width * math.sqrt(2 * g * abs(head_diff))

        return flow

    def _update_water_levels(self, dt: float):
        flow = self._calculate_flow_rate()
        self.flow_rate = abs(flow)

        chamber_area = self.gate_config.get("chamber_length", 60.0) * self.gate_config.get("chamber_width", 10.0)

        if self.phase == "charging":
            dv = flow * dt
            dh = dv / chamber_area
            self.water_level_up -= dh * 0.01
            self.water_level_down += dh * 0.01

            target_level = self.gate_config.get("max_water_level_up", 8.0) - 0.3
            if self.water_level_down >= target_level:
                self.phase = "passage"
                self.phase_start_time = time.time()
                self.gate_opening = 1.0
                self.flow_rate = 0.0

        elif self.phase == "discharging":
            dv = flow * dt
            dh = dv / chamber_area
            self.water_level_up += dh * 0.01
            self.water_level_down -= dh * 0.01

            target_level = self.gate_config.get("min_water_level_down", 3.0) + 0.3
            if self.water_level_up <= target_level:
                self.phase = "passage"
                self.phase_start_time = time.time()
                self.gate_opening = 1.0
                self.flow_rate = 0.0

        elif self.phase == "passage":
            self.flow_rate = 0.0
            if time.time() - self.phase_start_time > 300:
                self.phase = "idle"
                self.gate_opening = 0.0
                self.passage_count += 1

    def generate_reading(self) -> Dict:
        dt = 300

        if self.phase == "idle":
            if random.random() < 0.1:
                direction = random.choice(["upstream", "downstream"])
                if direction == "upstream":
                    self.phase = "charging"
                    self.gate_opening = 0.3 + random.random() * 0.5
                else:
                    self.phase = "discharging"
                    self.gate_opening = 0.3 + random.random() * 0.5
                self.phase_start_time = time.time()
        else:
            self._update_water_levels(dt)

        noise = random.gauss(0, 0.05)
        self.water_level_up += noise * 0.1
        self.water_level_down += noise * 0.05

        max_up = self.gate_config.get("max_water_level_up", 8.5)
        min_up = self.gate_config.get("min_water_level_up", 4.0)
        max_down = self.gate_config.get("max_water_level_down", 5.0)
        min_down = self.gate_config.get("min_water_level_down", 2.0)

        self.water_level_up = max(min_up, min(max_up, self.water_level_up))
        self.water_level_down = max(min_down, min(max_down, self.water_level_down))

        self.status = "normal"
        if random.random() < 0.02:
            self.status = "warning"
        if random.random() < 0.005:
            self.status = "fault"

        return {
            "time": datetime.now(timezone.utc).isoformat(),
            "gate_id": self.gate_id,
            "water_level_up": round(self.water_level_up, 3),
            "water_level_down": round(self.water_level_down, 3),
            "gate_opening": round(self.gate_opening, 3),
            "flow_rate": round(self.flow_rate, 3),
            "passage_time": round(time.time() - self.phase_start_time if self.phase != "idle" else 0.0, 1),
            "status": self.status
        }


class SensorNetworkSimulator:
    def __init__(self, num_gates: int = 36):
        self.num_gates = num_gates
        self.gates: List[DouGateSimulator] = []
        self._initialize_gates()

    def _initialize_gates(self):
        for i in range(1, self.num_gates + 1):
            config = {
                "gate_width": 5.5 + random.random() * 1.5,
                "gate_height": 4.0 + random.random() * 1.5,
                "max_water_level_up": 8.0 + random.random() * 1.5,
                "min_water_level_up": 3.5 + random.random() * 1.0,
                "max_water_level_down": 4.5 + random.random() * 1.5,
                "min_water_level_down": 1.5 + random.random() * 1.0,
                "chamber_length": 50.0 + random.random() * 30.0,
                "chamber_width": 8.0 + random.random() * 4.0,
                "discharge_coefficient": 0.6 + random.random() * 0.1,
            }
            self.gates.append(DouGateSimulator(i, config))

    def send_reading(self, reading: Dict):
        try:
            response = requests.post(
                f"{API_BASE_URL}/sensors",
                json=reading,
                timeout=5
            )
            if response.status_code == 201:
                print(f"Gate {reading['gate_id']}: data sent successfully")
                return True
            else:
                print(f"Gate {reading['gate_id']}: failed with status {response.status_code}")
                return False
        except Exception as e:
            print(f"Gate {reading['gate_id']}: error - {e}")
            return False

    def run_once(self):
        print(f"\n=== Sensor Reading Cycle at {datetime.now().strftime('%Y-%m-%d %H:%M:%S')} ===")
        success_count = 0
        for gate in self.gates:
            reading = gate.generate_reading()
            if self.send_reading(reading):
                success_count += 1
        print(f"Successfully sent {success_count}/{self.num_gates} readings")
        return success_count

    def run_continuous(self, interval: int = 300):
        print(f"Starting sensor simulator with {self.num_gates} gates")
        print(f"Reporting every {interval} seconds")
        print(f"API endpoint: {API_BASE_URL}")
        print("Press Ctrl+C to stop\n")

        try:
            while True:
                self.run_once()
                time.sleep(interval)
        except KeyboardInterrupt:
            print("\n\nSimulator stopped by user")


def main():
    import argparse

    parser = argparse.ArgumentParser(description="灵渠陡门传感器模拟器")
    parser.add_argument("--gates", type=int, default=36, help="陡门数量 (默认: 36)")
    parser.add_argument("--interval", type=int, default=300, help="上报间隔秒数 (默认: 300)")
    parser.add_argument("--api", type=str, default="http://localhost:8080/api", help="API地址")
    parser.add_argument("--once", action="store_true", help="只运行一次")
    parser.add_argument("--gate", type=int, default=0, help="指定单个陡门ID测试")

    args = parser.parse_args()

    global API_BASE_URL
    API_BASE_URL = args.api

    if args.gate > 0:
        sim = SensorNetworkSimulator(num_gates=args.gate)
        gate = sim.gates[args.gate - 1]
        print(f"\nSingle gate test - Gate {args.gate}")
        for i in range(10):
            reading = gate.generate_reading()
            print(json.dumps(reading, indent=2, ensure_ascii=False))
            time.sleep(0.5)
    elif args.once:
        sim = SensorNetworkSimulator(num_gates=args.gates)
        sim.run_once()
    else:
        sim = SensorNetworkSimulator(num_gates=args.gates)
        sim.run_continuous(interval=args.interval)


if __name__ == "__main__":
    main()
