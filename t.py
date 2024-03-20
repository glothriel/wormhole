import time
import requests

while True:
    apps = requests.get("http://localhost:8081/v1/apps").json()
    if len(apps) == 0:
        print("No app!")

        time.sleep(5)
        continue
    ep = "http://localhost:" + apps[0]["endpoint"].split(":")[1]
    try:
      if requests.get(ep, timeout=1).status_code == 200:
          print("Hurra")
      else:
          print("Nope")
    except requests.exceptions.ReadTimeout:
        print("Timeout!")
    
    time.sleep(0.2)
    
