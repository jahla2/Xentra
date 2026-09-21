FROM node:22-alpine AS build
WORKDIR /app
COPY package.json ./
RUN npm install
COPY . .
ARG VITE_XENTRA_API_URL=
ARG VITE_XENTRA_RUNNER_CONTROL_URL=
ENV VITE_XENTRA_API_URL=$VITE_XENTRA_API_URL
ENV VITE_XENTRA_RUNNER_CONTROL_URL=$VITE_XENTRA_RUNNER_CONTROL_URL
RUN npm run build

FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=3s --retries=3 CMD wget -q -O- http://127.0.0.1/healthz || exit 1
