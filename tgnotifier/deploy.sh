#!/bin/bash

# on update trigger
gcloud functions deploy telegram_notifier --region=europe-west2

# on create trigger
gcloud functions deploy telegram_notifier-1 --region=europe-west2