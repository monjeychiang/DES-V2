from abc import ABC, abstractmethod
from typing import Dict


class BaseStrategy(ABC):
    name: str

    @abstractmethod
    def on_tick(self, symbol: str, price: float, indicators: Dict[str, float]):
        """Return an action dict: {'action': 'BUY|SELL|HOLD', 'size': float, 'note': str}"""
        raise NotImplementedError

