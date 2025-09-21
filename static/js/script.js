document.addEventListener('DOMContentLoaded', function () {
    // --- Element Cache ---
    const form = document.getElementById('generate-form');
    const submitBtn = document.getElementById('submit-btn');
    const loadingIndicator = document.getElementById('loading');
    const resultContainer = document.getElementById('result-container');
    const imageUploadGroup = document.getElementById('image-upload-group');
    const imageUrlGroup = document.querySelector('label[for="imageUrl"]').parentElement;
    const imageUpload = document.getElementById('image-upload');
    const imageUrlInput = document.getElementById('imageUrl');
    const previewContainer = document.getElementById('image-preview-container');
    const preview = document.getElementById('preview');
    const clearImageBtn = document.getElementById('clear-image-btn');
    const stepsInput = document.getElementById('steps');
    const stepsValue = document.getElementById('steps-value');
    const modelSelect = document.getElementById('model');
    const dynamicParams = document.querySelectorAll('.dynamic-param');

    let modelsData = []; // To store the data from /api/models

    // --- 1. Dynamic Model & Parameter Loading ---
    fetch('/api/models')
        .then(response => response.json())
        .then(data => {
            modelsData = data;
            modelSelect.innerHTML = ''; // Clear existing options
            modelsData.forEach(provider => {
                const optgroup = document.createElement('optgroup');
                optgroup.label = provider.provider;
                provider.models.forEach(model => {
                    const option = document.createElement('option');
                    option.value = `${provider.provider}/${model.name}`;
                    option.textContent = model.name;
                    // Store supported params in a data attribute
                    option.dataset.params = JSON.stringify(model.supported_params);
                    optgroup.appendChild(option);
                });
                modelSelect.appendChild(optgroup);
            });
            // Trigger change event to set initial visibility
            modelSelect.dispatchEvent(new Event('change'));
        })
        .catch(error => {
            console.error('Error fetching models:', error);
            modelSelect.innerHTML = '<option value="">Error loading models</option>';
        });

    // --- 2. Update UI based on selected model ---
    modelSelect.addEventListener('change', function () {
        const selectedOption = this.options[this.selectedIndex];
        if (!selectedOption || !selectedOption.dataset.params) {
            return;
        }
        const supportedParams = JSON.parse(selectedOption.dataset.params);

        // Toggle visibility of dynamic parameter controls
        dynamicParams.forEach(paramEl => {
            const paramName = paramEl.dataset.param;
            if (supportedParams.includes(paramName)) {
                paramEl.classList.remove('hidden');
            } else {
                paramEl.classList.add('hidden');
            }
        });

        // Toggle visibility of image-related inputs
        if (supportedParams.includes('image')) {
            imageUploadGroup.classList.remove('hidden');
            imageUrlGroup.classList.remove('hidden');
        } else {
            imageUploadGroup.classList.add('hidden');
            imageUrlGroup.classList.add('hidden');
            clearImage(); // Clear any selected image if the model doesn't support it
        }
    });


    // --- 3. Handle Input Exclusivity ---
    imageUpload.addEventListener('change', function () {
        if (this.files && this.files[0]) {
            imageUrlInput.value = ''; // Clear URL input
            const reader = new FileReader();
            reader.onload = function (e) {
                preview.src = e.target.result;
                previewContainer.style.display = 'block';
                clearImageBtn.classList.remove('hidden');
            };
            reader.readAsDataURL(this.files[0]);
        }
    });

    imageUrlInput.addEventListener('input', function () {
        if (this.value) {
            clearFileInput(); // Clear file input if URL is typed
        }
    });

    function clearFileInput() {
        imageUpload.value = ''; // Clear the file input
        preview.src = '#';
        previewContainer.style.display = 'none';
        clearImageBtn.classList.add('hidden');
    }

    function clearImage() {
        clearFileInput();
        imageUrlInput.value = ''; // Clear the image URL input
    }

    clearImageBtn.addEventListener('click', clearImage);

    // --- 4. Form Submission ---
    form.addEventListener('submit', function (e) {
        e.preventDefault();

        const formData = new FormData(form);

        submitBtn.disabled = true;
        loadingIndicator.classList.remove('hidden');
        resultContainer.innerHTML = '<p>正在生成中，请稍候...</p>';

        fetch('/api/generate', {
            method: 'POST',
            body: formData
        })
            .then(response => {
                if (!response.ok) {
                    return response.text().then(text => { throw new Error(text) });
                }
                return response.json();
            })
            .then(data => {
                resultContainer.innerHTML = `<img src="${data.imageUrl}" alt="Generated Image">`;
            })
            .catch(error => {
                console.error('Error:', error);
                resultContainer.innerHTML = `<p class="error">生成失败: ${error.message}</p>`;
            })
            .finally(() => {
                submitBtn.disabled = false;
                loadingIndicator.classList.add('hidden');
            });
    });

    // --- 5. UI Helpers ---
    stepsInput.addEventListener('input', function () {
        stepsValue.textContent = this.value;
    });
});