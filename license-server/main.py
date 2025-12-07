import os
from datetime import datetime, timedelta

import jwt
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

app = FastAPI(title="DES License Server")

SECRET = os.getenv("LICENSE_SECRET", "dev-secret")


class LicenseRequest(BaseModel):
    machine: str
    days: int = 30


class LicenseResponse(BaseModel):
    token: str
    expires_at: datetime


@app.post("/license/issue", response_model=LicenseResponse)
def issue_license(req: LicenseRequest):
    exp = datetime.utcnow() + timedelta(days=req.days)
    payload = {"machine": req.machine, "exp": exp, "iat": datetime.utcnow()}
    token = jwt.encode(payload, SECRET, algorithm="HS256")
    return LicenseResponse(token=token, expires_at=exp)


@app.get("/health")
def health():
    return {"status": "ok"}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8000)

