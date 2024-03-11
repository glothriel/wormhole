import time
import requests

while True:
    time.sleep(5)
    apps = requests.get("https://karagiorgis.pl/v1/apps").json()
    if len(apps) == 0:
        print("No app!")
        continue
    ep = "http://karagiorgis.pl:" + apps[0]["endpoint"].split(":")[1]
    try:
      if requests.get(ep, timeout=1).status_code == 200:
          print("Hurra")
      else:
          print("Nope")
    except requests.exceptions.ReadTimeout:
        print("Timeout!")
    
