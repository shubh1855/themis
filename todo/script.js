document.addEventListener('DOMContentLoaded', () => {
    const todoInput = document.getElementById('todo-input');
    const addBtn = document.getElementById('add-btn');
    const todoList = document.getElementById('todo-list');

    const addTask = () => {
        const text = todoInput.value.trim();
        if (text === '') return;

        const li = document.createElement('li');
        
        const span = document.createElement('span');
        span.className = 'todo-text';
        span.textContent = text;
        span.onclick = () => span.classList.toggle('completed');

        const actions = document.createElement('div');
        actions.className = 'actions';

        const completeBtn = document.createElement('button');
        completeBtn.textContent = '✓';
        completeBtn.className = 'btn-action btn-complete';
        completeBtn.onclick = () => span.classList.toggle('completed');

        const deleteBtn = document.createElement('button');
        deleteBtn.textContent = '✕';
        deleteBtn.className = 'btn-action btn-delete';
        deleteBtn.onclick = () => {
            li.style.opacity = '0';
            li.style.transform = 'translateX(20px)';
            li.style.transition = 'all 0.3s ease';
            setTimeout(() => li.remove(), 300);
        };

        actions.appendChild(completeBtn);
        actions.appendChild(deleteBtn);

        li.appendChild(span);
        li.appendChild(actions);
        todoList.appendChild(li);

        todoInput.value = '';
        todoInput.focus();
    };

    addBtn.addEventListener('click', addTask);

    todoInput.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            addTask();
        }
    });
});