# Используем официальный образ Python с поддержкой CPU (без CUDA)
FROM python:3.10-slim

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем файл с зависимостями
COPY requirements.txt ./requirements.txt

# Устанавливаем зависимости
RUN pip install --no-cache-dir -r requirements.txt

# Копируем всё содержимое папки ml в рабочую директорию
COPY ./classify_app.py ./classify_app.py

# Указываем порт, который будет слушать приложение
EXPOSE 8000

# Запускаем uvicorn (без --reload для продакшена)
CMD ["uvicorn", "classify_app:app", "--host", "0.0.0.0", "--port", "8000"]
