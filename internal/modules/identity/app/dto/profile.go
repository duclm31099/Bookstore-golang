package dto

import app_query "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/query"

type GetMeOutput = app_query.MeView
type SessionOutput = app_query.SessionView
type DeviceOutput = app_query.DeviceView
type AddressOutput = app_query.AddressView
