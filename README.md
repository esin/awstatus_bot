# ☁️ AWStatus Telegram bot

For now, bot are living at [t.me/awstatus_bot](t.me/awstatus_bot)

AWS have a great status monitoring page - https://status.aws.amazon.com but it huge and can't notify about services issues, so appeared idea to write notification bot

This bot consists of 4 parts:

1. RSS list generator
2. RSS parser
3. Telegram dashboard 
4. Telegram notifier


### <a name="rsslist"></a>RSS list generator 
This tool deployed as a [Google Functions](https://cloud.google.com/functions) and parses every day status.aws.amazon.com for new services or regions and add them to [Cloud Firestore](https://firebase.google.com/docs/firestore)

### <a name="rssparser"></a>RSS parser
This tool deployed as a [Google Functions](https://cloud.google.com/functions) and parses every minute collections, which [RSS list generator](#rsslist) generated

### <a name="tgdash"></a>Telegram dashboard 
Simpliest Telegram bot, which can only receive command `/start` and put new user to database

### <a name="tgnotifier"></a>Telegram notifier
This tool deployed as a [Google Functions](https://cloud.google.com/functions) and triggered with [Cloud Firestore](https://firebase.google.com/docs/functions/firestore-events) on creating new items in collections, which RSS parser fills
Due that Cloud Firestore can trigger any function, here can be any endpoint (Slack, Webhook, etc.)
