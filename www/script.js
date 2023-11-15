var Prism = window.Prism || {};

// Функция для выполнения запроса на бэкенд и получения sessionID
async function fetchSessionID() {
    try {
        const response = await fetch('http://localhost:8080/');
        const data = await response.json();
        return data.sessionID;
    } catch (error) {
        console.error("Failed to fetch session ID:", error);
        return null;
    }
}

// Функция для выполнения редиректа на /live/sessionid
function redirectToLive(sessionID) {
    if (sessionID) {
        // Сохраняем sessionID в localStorage (или другом подходящем месте)
        localStorage.setItem('sessionID', sessionID);

        // Изменяем URL без перезагрузки страницы
        const newURL = `${window.location.origin}/live/${sessionID}`;
        window.history.pushState({ path: newURL }, '', newURL);
    } else {
        console.error("Invalid session ID");
    }
}

// Вызовите функции при загрузке страницы
document.addEventListener('DOMContentLoaded', async function () {
    const sessionID = await fetchSessionID();
    redirectToLive(sessionID);
});

var editor = CodeMirror(document.getElementById('code'), {
    value: "SELECT * FROM your_table;",
    mode: "text/x-sql",
    lineNumbers: true,
    lineWrapping: false,
    theme: "material-darker",
    styleActiveLine: true,
    viewportMargin: Infinity
});

var editorClass = document.getElementsByClassName("CodeMirror");
for (var i = 0; i < editorClass.length; i++) {
    editorClass[i].style.height = "100%";
}


function executeQuery() {
    var sqlCode = editor.getValue();
    // Ваш код для отправки запроса на сервер и обработки результата
    // Замените этот код на реальный код обращения к вашему серверу
    // и обработки ответа, например, с использованием AJAX
    document.getElementById("output").innerText = "Выполняется запрос:\n" + sqlCode;
}

Prism.highlightAll();


document.addEventListener('DOMContentLoaded', async function () {
    const sessionID = await fetchSessionID();

    // Установка соединения с WebSocket
    const socket = new WebSocket(`ws://localhost:8080/live/${sessionID}`);



    // Обработчики событий WebSocket
    socket.onopen = function () {
        console.log("Соединение по WebSocket открыто.");
    };

    socket.onmessage = function (event) {
        const data = JSON.parse(event.data);
        editor.setValue(data.code);
    };

    socket.onclose = function (event) {
        if (event.wasClean) {
            console.log(`Соединение закрыто, код=${event.code}, причина=${event.reason}`);
        } else {
            console.error('Соединение прервано');
        }
    };

    socket.onerror = function (error) {
        console.error(`Ошибка WebSocket: ${error}`);
    };

    editor.on('change', function () {
        const code = editor.getValue();
        socket.send(JSON.stringify({ code }));
    });

    // Обработчик события для перехвата изменения URL
    window.onpopstate = function (event) {
        // Обрабатываем изменение URL, если необходимо
        const newPath = window.location.pathname;
        const newSessionID = newPath.split('/').pop();

        if (newSessionID) {
            // Устанавливаем новый sessionID и переподключаемся к WebSocket
            socket.close();
            socket = new WebSocket(`ws://localhost:8080/live/${newSessionID}`);
        }

        console.log('URL changed:', event.state);
    };

    // Очищаем localStorage после использования sessionID
    localStorage.removeItem('sessionID');
});