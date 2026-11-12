package docs

import "github.com/swaggo/swag"

const swaggerTemplate = `{
  "swagger": "2.0",
  "info": {
    "title": "{{.Title}}",
    "description": "{{.Description}}",
    "version": "{{.Version}}"
  },
  "basePath": "{{.BasePath}}",
  "schemes": ["http", "https"],
  "securityDefinitions": {
    "BearerAuth": {
      "type": "apiKey",
      "name": "Authorization",
      "in": "header",
      "description": "Enter: Bearer <JWT>"
    }
  },
  "paths": {
    "/healthz": {
      "get": {
        "summary": "Check API and database health",
        "tags": ["health"],
        "responses": {
          "200": {"description": "Service is healthy"},
          "503": {"description": "Database is unavailable"}
        }
      }
    },
    "/auth/register": {
      "post": {
        "summary": "Register a tenant account",
        "tags": ["authentication"],
        "parameters": [{"in":"body","name":"body","required":true,"schema":{"$ref":"#/definitions/RegisterRequest"}}],
        "responses": {
          "201": {"description":"Account created","schema":{"$ref":"#/definitions/AuthResponse"}},
          "400": {"description":"Invalid input"},
          "409": {"description":"Email or phone already registered"}
        }
      }
    },
    "/auth/login": {
      "post": {
        "summary": "Log in",
        "tags": ["authentication"],
        "parameters": [{"in":"body","name":"body","required":true,"schema":{"$ref":"#/definitions/LoginRequest"}}],
        "responses": {
          "200": {"description":"Authenticated","schema":{"$ref":"#/definitions/AuthResponse"}},
          "401": {"description":"Invalid credentials"}
        }
      }
    },
    "/me": {
      "get": {
        "summary": "Get current user profile",
        "tags": ["account"],
        "security": [{"BearerAuth":[]}],
        "responses": {"200":{"description":"User profile","schema":{"$ref":"#/definitions/User"}}}
      }
    },
    "/me/devices": {
      "get": {
        "summary": "List current user's registered devices",
        "tags": ["account"],
        "security": [{"BearerAuth":[]}],
        "responses": {"200":{"description":"Device list"}}
      },
      "post": {
        "summary": "Register or reactivate a device",
        "tags": ["account"],
        "security": [{"BearerAuth":[]}],
        "parameters": [{"in":"body","name":"body","required":true,"schema":{"$ref":"#/definitions/DeviceRequest"}}],
        "responses": {"200":{"description":"Registered device","schema":{"$ref":"#/definitions/UserDevice"}}}
      }
    },
    "/me/devices/{id}": {
      "patch": {
        "summary": "Activate or pause a registered device",
        "tags": ["account"],
        "security": [{"BearerAuth":[]}],
        "parameters": [
          {"in":"path","name":"id","required":true,"type":"string","format":"uuid"},
          {"in":"body","name":"body","required":true,"schema":{"$ref":"#/definitions/DeviceActiveRequest"}}
        ],
        "responses": {"200":{"description":"Device updated","schema":{"$ref":"#/definitions/UserDevice"}}}
      }
    },
    "/reminders": {
      "post": {
        "summary": "Create a reminder",
        "tags": ["reminders"],
        "security": [{"BearerAuth":[]}],
        "parameters": [{"in":"body","name":"body","required":true,"schema":{"$ref":"#/definitions/ReminderRequest"}}],
        "responses": {"201":{"description":"Reminder created","schema":{"$ref":"#/definitions/Reminder"}}}
      },
      "get": {
        "summary": "List current user's reminders",
        "tags": ["reminders"],
        "security": [{"BearerAuth":[]}],
        "parameters": [
          {"in":"query","name":"limit","type":"integer","minimum":1,"maximum":100},
          {"in":"query","name":"offset","type":"integer","minimum":0}
        ],
        "responses": {"200":{"description":"Reminder list"}}
      }
    },
    "/notification-logs": {
      "get": {
        "summary": "List current user's notification logs",
        "tags": ["notifications"],
        "security": [{"BearerAuth":[]}],
        "parameters": [
          {"in":"query","name":"limit","type":"integer","minimum":1,"maximum":100},
          {"in":"query","name":"offset","type":"integer","minimum":0}
        ],
        "responses": {"200":{"description":"Notification log list","schema":{"$ref":"#/definitions/NotificationLogList"}}}
      }
    },
    "/reminders/{id}": {
      "get": {
        "summary": "Get a reminder",
        "tags": ["reminders"],
        "security": [{"BearerAuth":[]}],
        "parameters": [{"in":"path","name":"id","required":true,"type":"string","format":"uuid"}],
        "responses": {"200":{"description":"Reminder","schema":{"$ref":"#/definitions/Reminder"}}}
      },
      "put": {
        "summary": "Replace reminder details",
        "tags": ["reminders"],
        "security": [{"BearerAuth":[]}],
        "parameters": [
          {"in":"path","name":"id","required":true,"type":"string","format":"uuid"},
          {"in":"body","name":"body","required":true,"schema":{"$ref":"#/definitions/ReminderRequest"}}
        ],
        "responses": {"200":{"description":"Reminder updated","schema":{"$ref":"#/definitions/Reminder"}}}
      },
      "delete": {
        "summary": "Delete a reminder",
        "tags": ["reminders"],
        "security": [{"BearerAuth":[]}],
        "parameters": [{"in":"path","name":"id","required":true,"type":"string","format":"uuid"}],
        "responses": {"204":{"description":"Reminder deleted"}}
      }
    },
    "/reminders/{id}/status": {
      "patch": {
        "summary": "Update reminder status",
        "tags": ["reminders"],
        "security": [{"BearerAuth":[]}],
        "parameters": [
          {"in":"path","name":"id","required":true,"type":"string","format":"uuid"},
          {"in":"body","name":"body","required":true,"schema":{"$ref":"#/definitions/ReminderStatusRequest"}}
        ],
        "responses": {"200":{"description":"Reminder status updated","schema":{"$ref":"#/definitions/Reminder"}}}
      }
    },
    "/admin/analytics": {
      "get": {
        "summary": "Get platform usage counts",
        "tags": ["admin"],
        "security": [{"BearerAuth":[]}],
        "responses": {"200":{"description":"Platform analytics"}}
      }
    },
    "/admin/users": {
      "get": {
        "summary": "List platform users",
        "tags": ["admin"],
        "security": [{"BearerAuth":[]}],
        "parameters": [
          {"in":"query","name":"limit","type":"integer","minimum":1,"maximum":100},
          {"in":"query","name":"offset","type":"integer","minimum":0}
        ],
        "responses": {"200":{"description":"User list"}}
      }
    },
    "/admin/users/{id}/subscription": {
      "patch": {
        "summary": "Update a user's subscription tier",
        "tags": ["admin"],
        "security": [{"BearerAuth":[]}],
        "parameters": [
          {"in":"path","name":"id","required":true,"type":"string","format":"uuid"},
          {"in":"body","name":"body","required":true,"schema":{"$ref":"#/definitions/SubscriptionRequest"}}
        ],
        "responses": {"200":{"description":"User updated","schema":{"$ref":"#/definitions/AdminUser"}}}
      }
    },
    "/admin/users/{id}/role": {
      "patch": {
        "summary": "Update a user's role",
        "tags": ["admin"],
        "security": [{"BearerAuth":[]}],
        "parameters": [
          {"in":"path","name":"id","required":true,"type":"string","format":"uuid"},
          {"in":"body","name":"body","required":true,"schema":{"$ref":"#/definitions/RoleRequest"}}
        ],
        "responses": {"200":{"description":"User updated","schema":{"$ref":"#/definitions/AdminUser"}}}
      }
    }
  },
  "definitions": {
    "RegisterRequest": {
      "type":"object","required":["email","phone_number","password"],
      "properties":{
        "email":{"type":"string","format":"email"},
        "phone_number":{"type":"string","example":"+919876543210"},
        "password":{"type":"string","minLength":12,"maxLength":72},
        "full_name":{"type":"string"}
      }
    },
    "LoginRequest": {
      "type":"object","required":["email","password"],
      "properties":{"email":{"type":"string","format":"email"},"password":{"type":"string"}}
    },
    "AuthResponse": {
      "type":"object",
      "properties":{"user":{"$ref":"#/definitions/User"},"token":{"type":"string"}}
    },
    "User": {
      "type":"object",
      "properties":{
        "id":{"type":"string","format":"uuid"},
        "email":{"type":"string","format":"email"},
        "phone_number":{"type":"string"},
        "full_name":{"type":"string"},
        "role":{"type":"string","enum":["super_admin","tenant_user"]},
        "subscription_tier":{"type":"string"},
        "created_at":{"type":"string","format":"date-time"}
      }
    },
    "AdminUser": {
      "type":"object",
      "properties":{
        "id":{"type":"string","format":"uuid"},
        "email":{"type":"string","format":"email"},
        "phone_number":{"type":"string"},
        "full_name":{"type":"string"},
        "role":{"type":"string","enum":["super_admin","tenant_user"]},
        "subscription_tier":{"type":"string"},
        "created_at":{"type":"string","format":"date-time"}
      }
    },
    "UserDevice": {
      "type":"object",
      "properties":{
        "id":{"type":"string","format":"uuid"},
        "device_token":{"type":"string"},
        "platform":{"type":"string","enum":["android","ios"]},
        "device_name":{"type":"string"},
        "is_active":{"type":"boolean"},
        "last_used_at":{"type":"string","format":"date-time"},
        "created_at":{"type":"string","format":"date-time"},
        "updated_at":{"type":"string","format":"date-time"}
      }
    },
    "DeviceRequest": {
      "type":"object","required":["device_token","platform"],
      "properties":{
        "device_token":{"type":"string"},
        "platform":{"type":"string","enum":["android","ios"]},
        "device_name":{"type":"string","maxLength":150}
      }
    },
    "DeviceActiveRequest": {
      "type":"object","required":["is_active"],
      "properties":{"is_active":{"type":"boolean"}}
    },
    "ReminderRequest": {
      "type":"object","required":["entity_name","start_date","recurrence"],
      "properties":{
        "entity_name":{"type":"string","maxLength":255},
        "description":{"type":"string"},
        "start_date":{"type":"string","format":"date","example":"2026-01-31"},
        "recurrence":{"type":"string","enum":["weekly","15_days","monthly","yearly","custom"]},
        "custom_interval_value":{"type":"integer","minimum":1,"maximum":1000},
        "custom_interval_unit":{"type":"string","enum":["days","weeks","months","years"]},
        "advance_notice_days":{"type":"integer","minimum":0,"maximum":365},
        "notification_time":{"type":"string","example":"09:00"},
        "timezone":{"type":"string","example":"Asia/Kolkata"}
      }
    },
    "Reminder": {
      "type":"object",
      "properties":{
        "id":{"type":"string","format":"uuid"},
        "user_id":{"type":"string","format":"uuid"},
        "entity_name":{"type":"string"},
        "description":{"type":"string"},
        "start_date":{"type":"string","format":"date"},
        "next_due_date":{"type":"string","format":"date"},
        "recurrence":{"type":"string","enum":["weekly","15_days","monthly","yearly","custom"]},
        "custom_interval_value":{"type":"integer"},
        "custom_interval_unit":{"type":"string","enum":["days","weeks","months","years"]},
        "advance_notice_days":{"type":"integer"},
        "notification_time":{"type":"string"},
        "timezone":{"type":"string"},
        "status":{"type":"string","enum":["active","paused","completed"]},
        "created_at":{"type":"string","format":"date-time"}
      }
    },
    "NotificationLog": {
      "type":"object",
      "properties":{
        "id":{"type":"string","format":"uuid"},
        "reminder_id":{"type":"string","format":"uuid"},
        "user_id":{"type":"string","format":"uuid"},
        "device_id":{"type":"string","format":"uuid"},
        "scheduled_for":{"type":"string","format":"date-time"},
        "sent_at":{"type":"string","format":"date-time"},
        "status":{"type":"string"},
        "channel":{"type":"string"},
        "provider":{"type":"string"},
        "provider_message_id":{"type":"string"},
        "error_message":{"type":"string"},
        "retry_count":{"type":"integer"},
        "created_at":{"type":"string","format":"date-time"}
      }
    },
    "NotificationLogList": {
      "type":"object",
      "properties":{
        "items":{"type":"array","items":{"$ref":"#/definitions/NotificationLog"}},
        "limit":{"type":"integer"},
        "offset":{"type":"integer"}
      }
    },
    "ReminderStatusRequest": {
      "type":"object","required":["status"],
      "properties":{"status":{"type":"string","enum":["active","paused","completed"]}}
    },
    "SubscriptionRequest": {
      "type":"object","required":["subscription_tier"],
      "properties":{"subscription_tier":{"type":"string","maxLength":20}}
    },
    "RoleRequest": {
      "type":"object","required":["role"],
      "properties":{"role":{"type":"string","enum":["tenant_user","super_admin"]}}
    }
  }
}`

var SwaggerInfo = &swag.Spec{
	Version:          "1.0",
	BasePath:         "/api/v1",
	Title:            "Reminder API",
	Description:      "Tenant-scoped recurring reminder REST API.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  swaggerTemplate,
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
