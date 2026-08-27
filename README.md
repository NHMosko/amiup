# amiup
### Simple uptimer
amiup strives to be reliable and small, solving the problem of keeping track of your applications' state, with no fancy features

## How to use it
amiup works with a postgres database set in a .env,
you create services for it to keep track of with http post requests,
you keep the server running and everything is done through http,
you may also set these optional env variables (recommended): 
 - an api key 
 - an allowed origin for the requests

when creating a service you give it:
 - a name
 - a URL (the link to the website that is checked)
 - timeout (how long it waits before giving up on the connection)
 - a heartbeat (how often it checks if it's up)
 - strikes (how many times it needs to be down before it is actually considered down)
 - a discord webhook (where it notifies you if the service is down)
all of which can be edited later

the program also keeps track of all requisitions if you wish to check for more information

the endpoints are:
- /services
- /services/{serviceID}
- /services/delete/{serviceID}
- /requisitions

all of the returned jsons in this version are not formatted.

Example POST to /services:
```curl -H "Authorization: ApiKey 1234" localhost:8082/services --data '{"name": "Google", "timeout": 10, "url": "https://google.com", "heartbeat": 5, "strikes": 3, "discord_webhook": ""}'```
