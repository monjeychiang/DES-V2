from typing import Dict

from strategies.base import BaseStrategy


class ExampleGrid(BaseStrategy):
    def __init__(self, symbol: str, lower: float, upper: float, size: float):
        self.name = f"grid_{symbol}"
        self.symbol = symbol
        self.lower = lower
        self.upper = upper
        self.size = size
        self.last_action = ""
        self.min_step_ratio = 0.002

    def on_tick(self, symbol: str, price: float, indicators: Dict[str, float]):
        if symbol and symbol != self.symbol:
            return None
        if price <= 0:
            return None

        if self.last_action == "BUY" and price > self.lower * (1 + self.min_step_ratio):
            self.last_action = ""
        if self.last_action == "SELL" and price < self.upper * (1 - self.min_step_ratio):
            self.last_action = ""

        if price <= self.lower and self.last_action != "BUY":
            self.last_action = "BUY"
            return {"action": "BUY", "symbol": self.symbol, "size": self.size, "note": f"grid buy {price:.2f}"}

        if price >= self.upper and self.last_action != "SELL":
            self.last_action = "SELL"
            return {"action": "SELL", "symbol": self.symbol, "size": self.size, "note": f"grid sell {price:.2f}"}

        return None

