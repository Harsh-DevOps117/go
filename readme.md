# NoteFlow - Smart Notes & Alarm Management Platform

A production-ready Note and Reminder Management Platform built with Golang, Gin, PostgreSQL, Redis, JWT Authentication, and Docker.

The application allows users to securely create notes, schedule reminders, manage alarms, and access their data through a scalable REST API architecture.

---

## Features

### Authentication & Security

- User Registration
- User Login
- User Logout
- JWT Access Tokens
- JWT Refresh Tokens
- Password Hashing with bcrypt
- Protected Routes
- Role-Based Access Control (RBAC)
- Middleware-Based Authorization

---

### Notes Management

- Create Notes
- Read Notes
- Update Notes
- Delete Notes
- User-Owned Resources
- Ownership Validation
- Redis Caching

---

### Alarm & Reminder System

- Create Alarms
- Update Alarms
- Delete Alarms
- Retrieve User Alarms
- Note-Based Reminder System
- Scheduled Reminder Execution
- Redis Sorted Set Scheduling Queue

---

### Admin Features

- View Users
- Delete Users
- Platform Statistics
- Alarm Monitoring
- User Management

---

### Performance & Scalability

- PostgreSQL Persistent Storage
- Redis Caching Layer
- Redis Session Storage
- Redis Alarm Queue
- Optimized Database Queries
- Database Indexing
- Stateless JWT Authentication

---

### DevOps & Infrastructure

- Dockerized Application
- Docker Compose Setup
- GitHub Actions CI/CD
- Structured Logging
- Health Monitoring
- Prometheus Metrics Support
- Production Ready Deployment

---

## Architecture

```text
Client
   |
   v
Gin Router
   |
   v
Authentication Middleware
   |
   v
Controllers
   |
   v
Services
   |
   +--------------------+
   |                    |
   v                    v
Redis Cache       PostgreSQL
(Cache Layer)     (Source of Truth)
```

---

## Tech Stack

### Backend

- Golang
- Gin
- GORM
- PostgreSQL
- Redis
- JWT
- bcrypt

### Frontend

- React
- TypeScript
- Tailwind CSS
- React Query
- React Router

### DevOps

- Docker
- Docker Compose
- GitHub Actions
- Prometheus
- Grafana

---

## Database Design

### Users

```sql
users
```

| Field      | Type      |
| ---------- | --------- |
| id         | uint      |
| username   | varchar   |
| email      | varchar   |
| password   | varchar   |
| role       | varchar   |
| created_at | timestamp |
| updated_at | timestamp |

---

### Notes

```sql
notes
```

| Field      | Type      |
| ---------- | --------- |
| id         | uint      |
| user_id    | uint      |
| title      | varchar   |
| content    | text      |
| created_at | timestamp |
| updated_at | timestamp |

---

### Alarms

```sql
alarms
```

| Field      | Type      |
| ---------- | --------- |
| id         | uint      |
| user_id    | uint      |
| note_id    | uint      |
| alarm_time | timestamp |
| status     | varchar   |
| created_at | timestamp |
| updated_at | timestamp |

---

### Refresh Tokens

```sql
refresh_tokens
```

| Field      | Type      |
| ---------- | --------- |
| id         | uint      |
| user_id    | uint      |
| token      | text      |
| created_at | timestamp |

---

## Authentication Flow

```text
Register
    ↓
Store User
    ↓
Login
    ↓
Validate Credentials
    ↓
Generate Access Token
    ↓
Generate Refresh Token
    ↓
Store Session
    ↓
Access Protected Routes
```

---

## Notes Flow

```text
Create Note
    ↓
Store in PostgreSQL
    ↓
Cache in Redis

Read Note
    ↓
Check Redis
    ↓
Cache Hit → Return
    ↓
Cache Miss
    ↓
PostgreSQL
    ↓
Update Cache
```

---

## Alarm Flow

```text
Create Alarm
    ↓
Validate Note Ownership
    ↓
Store Alarm
    ↓
Add to Redis Queue
    ↓
Scheduler Worker
    ↓
Trigger Alarm
    ↓
Notification Service
```

---

## API Endpoints

### Authentication

```http
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/logout
POST /api/v1/auth/refresh
```

### Notes

```http
POST   /api/v1/notes
GET    /api/v1/notes
GET    /api/v1/notes/:id
PUT    /api/v1/notes/:id
DELETE /api/v1/notes/:id
```

### Alarms

```http
POST   /api/v1/alarms
GET    /api/v1/alarms
GET    /api/v1/alarms/:id
PUT    /api/v1/alarms/:id
DELETE /api/v1/alarms/:id
```

### Admin

```http
GET    /api/v1/admin/users
DELETE /api/v1/admin/users/:id
GET    /api/v1/admin/stats
```

---

## Project Structure

```text
.
├── cmd
│   └── internal
│       └── main.go
│
├── controller
├── services
├── repository
├── middleware
├── models
├── dto
├── db
├── utils
│
├── docker-compose.yml
├── Dockerfile
├── .env
│
├── .github
│   └── workflows
│
└── README.md
```

---

## Environment Variables

```env
PORT=3000

SECRET_KEY_ACCESSTOKEN=your_access_secret
SECRET_KEY_REFRESH=your_refresh_secret

DB_HOST=postgres
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=notesdb
DB_SSLMODE=disable

REDIS_ADDR=redis:6379
REDIS_PASSWORD=
```

---

## Running Locally

### Clone Repository

```bash
git clone <repository-url>
cd project
```

### Start Services

```bash
docker compose up --build
```

### Health Check

```bash
GET http://localhost:3000/health
```

---

## Future Improvements

- Email Notifications
- SMS Notifications
- WebSocket Real-Time Events
- Google OAuth
- GitHub OAuth
- Multi-Tenant Support
- Kubernetes Deployment
- Distributed Scheduler
- Event-Driven Architecture
- Microservice Migration
- AI-Powered Note Summaries
- Semantic Search
- Vector Database Integration

---

## License

MIT License

---

Built with Golang, PostgreSQL, Redis, and Docker following scalable backend engineering principles.
