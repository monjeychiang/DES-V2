"""
Placeholder alert service. Extend to gRPC server if needed.
"""
import logging

from alert.telegram import TelegramNotifier


def send_test():
    notifier = TelegramNotifier()
    notifier.send("DES v2 alert service is running.")


if __name__ == "__main__":
    logging.basicConfig(level=logging.INFO)
    send_test()

